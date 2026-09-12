package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"finance-monitor/backend/internal/parser"
)

type AIFallbackParser interface {
	ParseNotification(ctx context.Context, input parser.Input) (*parser.Result, error)
}

type OpenRouterAIFallbackParser struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
	timeout  time.Duration
}

const aiParserEndpoint = "https://openrouter.ai/api/v1/chat/completions"

func NewOpenRouterAIFallbackParser(apiKey, model string) *OpenRouterAIFallbackParser {
	return NewOpenRouterAIFallbackParserWithClient(apiKey, model, aiParserEndpoint, &http.Client{Timeout: 5 * time.Second}, 3*time.Second)
}

func NewOpenRouterAIFallbackParserWithClient(apiKey, model, endpoint string, client *http.Client, timeout time.Duration) *OpenRouterAIFallbackParser {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "openrouter/free"
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &OpenRouterAIFallbackParser{
		apiKey:   strings.TrimSpace(apiKey),
		model:    model,
		endpoint: endpoint,
		client:   client,
		timeout:  timeout,
	}
}

const aiParserSystemPrompt = `You are a financial notification parser assistant.
Analyze incoming smartphone push notifications from Indonesian banking and financial apps (e.g. SeaBank, ShopeePay, Bank Jago, Mandiri, BRI, Flip).
Determine whether the notification represents an ACTUAL financial transaction (money moved, debited, credited, transfer, payment, refund) or if it is a NON-TRANSACTION / PROMOTION / CHAT / REMINDER / FAILED TRANSACTION.

Return ONLY a valid JSON object matching this schema:
{
  "is_financial_transaction": boolean,
  "type": "income" | "expense" | "transfer" | null,
  "amount": integer (in Indonesian Rupiah, e.g. 25000),
  "source_account": string | null,
  "destination_account": string | null,
  "merchant": string | null,
  "description": string | null,
  "confidence": float (between 0.0 and 1.0),
  "reason": string
}

Rules:
1. If the notification is a promotion, voucher, promo deal, driver chat, system security alert, bill reminder without payment, or a FAILED transaction (saldo tidak cukup, transaksi gagal, dibatalkan):
   Set "is_financial_transaction": false, "type": null, "amount": 0, and explain in "reason".
2. If the notification is a real completed financial transaction:
   Set "is_financial_transaction": true.
   - "type" MUST be "income", "expense", or "transfer".
   - "amount" MUST be a positive integer in IDR (e.g. convert "Rp 15.000" to 15000).
   - "source_account" / "destination_account": Standard owned account names if recognized ("SeaBank", "ShopeePay", "Bank Jago", "Mandiri", "BRI", "Flip").
   - "merchant": Counterparty name, store name, or recipient/sender name.
   - "confidence": Confidence score between 0.0 and 1.0.`

type aiParserPayload struct {
	IsFinancialTransaction bool    `json:"is_financial_transaction"`
	Type                   *string `json:"type"`
	Amount                 int64   `json:"amount"`
	SourceAccount          *string `json:"source_account"`
	DestinationAccount     *string `json:"destination_account"`
	Merchant               *string `json:"merchant"`
	Description            *string `json:"description"`
	Confidence             float64 `json:"confidence"`
	Reason                 string  `json:"reason"`
}

func (p *OpenRouterAIFallbackParser) ParseNotification(ctx context.Context, input parser.Input) (*parser.Result, error) {
	if p == nil || p.apiKey == "" {
		return nil, fmt.Errorf("openrouter api key unconfigured")
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	userPrompt := fmt.Sprintf("App: %s\nTitle: %s\nBody: %s", input.SourceApp, input.Title, input.Text)

	body, err := json.Marshal(struct {
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
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: &struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter status %d", resp.StatusCode)
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&completion); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("empty choice content from ai")
	}

	var parsed aiParserPayload
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal completion json: %w", err)
	}

	// Case 1: AI confirms it's a promotion or non-transaction
	if !parsed.IsFinancialTransaction {
		conf := parsed.Confidence
		if conf <= 0 {
			conf = 0.90
		}
		return &parser.Result{
			Ignore:      true,
			ParseStatus: "IGNORED_AI_NON_TRANSACTION",
			Confidence:  conf,
		}, nil
	}

	// Case 2: AI identified a real transaction
	if parsed.Type == nil || parsed.Amount <= 0 {
		return nil, fmt.Errorf("ai identified transaction but missing valid type or positive amount")
	}

	txType := strings.ToLower(strings.TrimSpace(*parsed.Type))
	if txType != "income" && txType != "expense" && txType != "transfer" {
		return nil, fmt.Errorf("invalid transaction type %q from ai", txType)
	}

	conf := parsed.Confidence
	if conf <= 0 || conf > 1.0 {
		conf = 0.85
	}

	res := &parser.Result{
		Type:        txType,
		Amount:      parsed.Amount,
		ParseStatus: "AUTO",
		Confidence:  conf,
	}

	if parsed.Merchant != nil {
		res.Merchant = strings.TrimSpace(*parsed.Merchant)
	}
	if parsed.Description != nil {
		res.Description = strings.TrimSpace(*parsed.Description)
	}
	if parsed.SourceAccount != nil {
		res.SourceAccountName = strings.TrimSpace(*parsed.SourceAccount)
	}
	if parsed.DestinationAccount != nil {
		res.DestinationAccountName = strings.TrimSpace(*parsed.DestinationAccount)
	}

	// Default fallback accounts if AI omitted them
	if txType == "expense" && res.SourceAccountName == "" {
		res.SourceAccountName = parser.AccountFromSource(input.SourceApp)
	}
	if txType == "income" && res.DestinationAccountName == "" {
		res.DestinationAccountName = parser.AccountFromSource(input.SourceApp)
	}

	return res, nil
}
