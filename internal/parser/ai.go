package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AIParser interface {
	Parse(ctx context.Context, input Input) (*Result, error)
}

type OpenRouterParser struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

const defaultAIEndpoint = "https://openrouter.ai/api/v1/chat/completions"

func NewOpenRouterParser(apiKey, model string) *OpenRouterParser {
	return NewOpenRouterParserWithEndpoint(apiKey, model, defaultAIEndpoint, &http.Client{Timeout: 15 * time.Second})
}

func NewOpenRouterParserWithEndpoint(apiKey, model, endpoint string, client *http.Client) *OpenRouterParser {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "openrouter/free"
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &OpenRouterParser{
		apiKey:   strings.TrimSpace(apiKey),
		model:    model,
		endpoint: endpoint,
		client:   client,
	}
}

const aiParserSystemPrompt = `You are an expert Indonesian banking and financial transaction parser.
Analyze the transaction text, which may be raw OCR text from a transaction receipt, payment screen, or transaction details (such as Bank Jago, SeaBank, BCA, Livin Mandiri, GoPay, ShopeePay, OVO, Dana).

Extract the transaction information and output ONLY a valid JSON object matching this schema:
{
  "type": "expense" | "income" | "transfer",
  "amount": number,
  "merchant": string,
  "source_account": string,
  "destination_account": string,
  "description": string,
  "category": string,
  "confidence": number
}

Rules:
1. "type":
   - "expense": money paid out, purchase, QRIS, transfer to another person ("uang keluar", "-Rp", "pembayaran", "debit").
   - "income": money received from another person or institution ("uang masuk", "+Rp", "kredit", "dana masuk", "terima uang").
   - "transfer": transfer between own accounts (e.g., from Bank Jago to own SeaBank/BCA account).
2. "amount":
   - Clean absolute integer in IDR (no dots, commas, or currency prefix).
   - E.g., "-Rp9.300.000" -> 9300000, "+Rp4.100.000" -> 4100000, "Rp 18.500" -> 18500.
3. "merchant":
   - For expense: the recipient person or store/merchant name (e.g. "MUHAMAD FATONY ANWAR", "Indomaret", "Kopi Kenangan").
   - For income: the sender person or institution (e.g. "MUHAMMAD IKHLAS RAMADHAN", "Shopee").
   - Clean up bank account numbers or extra words (e.g. if text is "MUHAMAD FATONY ANWAR BCA 1040668272", merchant is "MUHAMAD FATONY ANWAR").
   - Do NOT use generic words like "Transaksi", "Uang keluar", "Uang masuk", "Rincian Transaksi", "Kantong Utama".
   - If counterparty cannot be determined, return "".
4. "source_account" & "destination_account":
   - Standardize Indonesian bank/wallet names: "Bank Jago", "SeaBank", "BCA", "Mandiri", "BRI", "ShopeePay", "GoPay", "OVO", "Dana".
   - Note: "Kantong Utama" or ID with "JAG" belongs to "Bank Jago".
   - For expense: source_account is where money originated (e.g. "Bank Jago"), destination_account is recipient's bank if known.
   - For income: destination_account is where money entered (e.g. "Bank Jago"), source_account is sender's bank if known.
5. "description":
   - Any transaction note or remarks present, or a concise summary like "Transfer ke MUHAMAD FATONY ANWAR".
6. "category":
   - Suggested category, e.g. "Makanan & Minuman", "Pemasukan", "Belanja", "Transportasi", "Tagihan", or "Belum Dikategorikan".
7. "confidence":
   - Float between 0.0 and 1.0 (e.g. 0.95).`

type aiParseJSONResponse struct {
	Type               string  `json:"type"`
	Amount             int64   `json:"amount"`
	Merchant           string  `json:"merchant"`
	SourceAccount      string  `json:"source_account"`
	DestinationAccount string  `json:"destination_account"`
	Description        string  `json:"description"`
	Category           string  `json:"category"`
	Confidence         float64 `json:"confidence"`
}

func (p *OpenRouterParser) Parse(ctx context.Context, input Input) (*Result, error) {
	if p == nil || p.apiKey == "" {
		return nil, errors.New("openrouter api key unconfigured")
	}

	rawText := strings.TrimSpace(input.Text)
	if input.BigText != "" {
		rawText += "\n" + strings.TrimSpace(input.BigText)
	}
	if rawText == "" && input.Title != "" {
		rawText = input.Title
	}
	if rawText == "" {
		return nil, errors.New("empty input text for ai parser")
	}

	userContent := fmt.Sprintf("Source App: %s\nTitle: %s\nText:\n%s", input.SourceApp, input.Title, rawText)

	requestPayload := struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		ResponseFormat *struct {
			Type string `json:"type"`
		} `json:"response_format,omitempty"`
	}{
		Model: p.model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "system", Content: aiParserSystemPrompt},
			{Role: "user", Content: userContent},
		},
		ResponseFormat: &struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	}

	reqBody, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal ai parser request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create ai parser request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute ai parser request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("openrouter parser status %d: %s", resp.StatusCode, string(respBytes))
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode ai parser response: %w", err)
	}

	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return nil, errors.New("empty choice in ai parser response")
	}

	content := response.Choices[0].Message.Content
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var parsed aiParseJSONResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal ai parser json: %w", err)
	}

	if parsed.Amount <= 0 {
		if amt, ok := extractBestAmount(rawText); ok && amt > 0 {
			parsed.Amount = amt
		} else {
			return nil, errors.New("ai parser could not determine valid amount")
		}
	}

	tType := strings.ToLower(strings.TrimSpace(parsed.Type))
	if tType != "expense" && tType != "income" && tType != "transfer" {
		tType = "expense"
	}

	sourceAcc := accountFromSource(parsed.SourceAccount)
	if !isOwnedAccount(sourceAcc) {
		sourceAcc = detectOwnedAccount(parsed.SourceAccount)
	}
	if !isOwnedAccount(sourceAcc) {
		sourceAcc = detectOwnedAccount(rawText)
	}
	if !isOwnedAccount(sourceAcc) && isOwnedAccount(accountFromSource(input.SourceApp)) {
		sourceAcc = accountFromSource(input.SourceApp)
	}

	destAcc := accountFromSource(parsed.DestinationAccount)
	if !isOwnedAccount(destAcc) {
		destAcc = detectOwnedAccount(parsed.DestinationAccount)
	}

	confidence := parsed.Confidence
	if confidence <= 0 {
		confidence = 0.90
	}

	categoryName := strings.TrimSpace(parsed.Category)
	if categoryName == "" {
		if tType == "income" {
			categoryName = "Pemasukan"
		} else {
			categoryName = "Belum Dikategorikan"
		}
	}

	return &Result{
		Type:                   tType,
		Amount:                 parsed.Amount,
		SourceAccountName:      sourceAcc,
		DestinationAccountName: destAcc,
		Merchant:               strings.TrimSpace(parsed.Merchant),
		CategoryName:           categoryName,
		Description:            strings.TrimSpace(parsed.Description),
		ParseStatus:            "AUTO",
		Confidence:             confidence,
	}, nil
}
