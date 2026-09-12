package notification

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"finance-monitor/backend/internal/parser"
)

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func newTestClient(fn roundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
		Timeout:   2 * time.Second,
	}
}

func TestAIFallbackParserDetectsNonTransaction(t *testing.T) {
	mockResponse := `{"choices":[{"message":{"content":"{\"is_financial_transaction\": false, \"reason\": \"Push promo banner\", \"confidence\": 0.98}"}}]}`
	client := newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(mockResponse)),
			Header:     make(http.Header),
		}
	})

	aiParser := NewOpenRouterAIFallbackParserWithClient("test-key", "test-model", "https://api.test/v1", client, 1*time.Second)
	result, err := aiParser.ParseNotification(context.Background(), parser.Input{
		SourceApp: "ShopeePay",
		Title:     "Promo Baru",
		Text:      "Klaim voucher cashback 50%",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.Ignore || result.ParseStatus != "IGNORED_AI_NON_TRANSACTION" {
		t.Fatalf("expected ignored non-transaction, got %#v", result)
	}
}

func TestAIFallbackParserExtractsTransaction(t *testing.T) {
	mockResponse := `{"choices":[{"message":{"content":"{\"is_financial_transaction\": true, \"type\": \"expense\", \"amount\": 45000, \"source_account\": \"SeaBank\", \"merchant\": \"Warung Kopi Baru\", \"confidence\": 0.95}"}}]}`
	client := newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(mockResponse)),
			Header:     make(http.Header),
		}
	})

	aiParser := NewOpenRouterAIFallbackParserWithClient("test-key", "test-model", "https://api.test/v1", client, 1*time.Second)
	result, err := aiParser.ParseNotification(context.Background(), parser.Input{
		SourceApp: "SeaBank",
		Title:     "Transaksi Berhasil",
		Text:      "Pembayaran ke Warung Kopi Baru Rp45.000 sukses",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Type != "expense" || result.Amount != 45000 || result.Merchant != "Warung Kopi Baru" || result.SourceAccountName != "SeaBank" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestAIFallbackParserHandlesAPIError(t *testing.T) {
	client := newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader(`{"error": "rate limited"}`)),
			Header:     make(http.Header),
		}
	})

	aiParser := NewOpenRouterAIFallbackParserWithClient("test-key", "test-model", "https://api.test/v1", client, 1*time.Second)
	result, err := aiParser.ParseNotification(context.Background(), parser.Input{
		SourceApp: "Unknown",
		Title:     "Some Notification",
		Text:      "Text",
	})

	if err == nil {
		t.Fatal("expected error on 429 status code, got nil")
	}
	if result != nil {
		t.Fatalf("expected nil result on error, got %#v", result)
	}
}
