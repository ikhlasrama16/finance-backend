package report

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

const openRouterEndpoint = "https://openrouter.ai/api/v1/chat/completions"

var (
	ErrAIUnavailable  = errors.New("AI report is unavailable")
	ErrAIUnconfigured = errors.New("OPENROUTER_API_KEY is not configured")
)

type Generator interface {
	Generate(context.Context, string) (string, error)
	Model() string
}

type OpenRouterClient struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

func NewOpenRouterClient(apiKey, model string) *OpenRouterClient {
	return NewOpenRouterClientWithEndpoint(apiKey, model, openRouterEndpoint, &http.Client{Timeout: 45 * time.Second})
}

func NewOpenRouterClientWithEndpoint(apiKey, model, endpoint string, client *http.Client) *OpenRouterClient {
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	return &OpenRouterClient{apiKey: strings.TrimSpace(apiKey), model: model, endpoint: endpoint, client: client}
}

func (c *OpenRouterClient) Model() string { return c.model }

func (c *OpenRouterClient) Generate(ctx context.Context, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", ErrAIUnconfigured
	}
	body, err := json.Marshal(struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}{
		Model: c.model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("encode OpenRouter request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create OpenRouter request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrAIUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: OpenRouter returned HTTP %d", ErrAIUnavailable, response.StatusCode)
	}
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("%w: invalid OpenRouter response", ErrAIUnavailable)
	}
	if len(payload.Choices) == 0 || strings.TrimSpace(payload.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("%w: missing completion content", ErrAIUnavailable)
	}
	return strings.TrimSpace(payload.Choices[0].Message.Content), nil
}
