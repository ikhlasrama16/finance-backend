package category

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ClassifyInput struct {
	Type              string   `json:"type"`
	Merchant          string   `json:"merchant"`
	Description       string   `json:"description,omitempty"`
	Amount            int64    `json:"amount"`
	AllowedCategories []string `json:"allowed_categories"`
}

type ClassifyResult struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

type Classifier interface {
	Classify(ctx context.Context, input ClassifyInput) (*ClassifyResult, error)
}

type OpenRouterClassifier struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

const classifierEndpoint = "https://openrouter.ai/api/v1/chat/completions"

func NewOpenRouterClassifier(apiKey, model string) *OpenRouterClassifier {
	return NewOpenRouterClassifierWithEndpoint(apiKey, model, classifierEndpoint, &http.Client{Timeout: 10 * time.Second})
}

func NewOpenRouterClassifierWithEndpoint(apiKey, model, endpoint string, client *http.Client) *OpenRouterClassifier {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "openrouter/free"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &OpenRouterClassifier{
		apiKey:   strings.TrimSpace(apiKey),
		model:    model,
		endpoint: endpoint,
		client:   client,
	}
}

const classifierSystemPrompt = `You are a financial transaction category classifier.
Select EXACTLY one existing category from the allowed_categories list provided in the prompt.
Return ONLY valid JSON matching this schema:
{"category": string, "confidence": float, "reason": string}
Rules:
- Do NOT invent new category names.
- The returned category MUST be present in allowed_categories.
- If merchant is a person's name or unknown counterparty, do NOT guess confidently; return "Belum Dikategorikan" with low confidence (< 0.50).
- Confidence must be between 0.0 and 1.0.`

func (c *OpenRouterClassifier) Classify(ctx context.Context, input ClassifyInput) (*ClassifyResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("openrouter api key unconfigured")
	}
	if len(input.AllowedCategories) == 0 {
		return nil, fmt.Errorf("no allowed categories provided")
	}

	payloadJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

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
		Model: c.model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "system", Content: classifierSystemPrompt},
			{Role: "user", Content: string(payloadJSON)},
		},
		ResponseFormat: &struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter status %d", resp.StatusCode)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(payload.Choices) == 0 || strings.TrimSpace(payload.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("empty choice content")
	}

	var result ClassifyResult
	if err := json.Unmarshal([]byte(payload.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("unmarshal completion json: %w", err)
	}

	result.Category = strings.TrimSpace(result.Category)
	allowed := false
	for _, cat := range input.AllowedCategories {
		if strings.EqualFold(cat, result.Category) {
			result.Category = cat
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("returned category %q not in allowed set", result.Category)
	}

	return &result, nil
}
