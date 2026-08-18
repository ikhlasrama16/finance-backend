package category

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenRouterClassifier_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"category\": \"Makanan & Minuman\", \"confidence\": 0.95, \"reason\": \"Food vendor\"}"
				}
			}]
		}`))
	}))
	defer server.Close()

	classifier := NewOpenRouterClassifierWithEndpoint("test-key", "test-model", server.URL, server.Client())
	res, err := classifier.Classify(context.Background(), ClassifyInput{
		Type:              "expense",
		Merchant:          "BAKSO ZAKI WONOGIRI",
		AllowedCategories: []string{"Belum Dikategorikan", "Makanan & Minuman", "Laundry"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Category != "Makanan & Minuman" || res.Confidence != 0.95 {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestOpenRouterClassifier_InvalidCategoryRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"category\": \"Nonexistent Category\", \"confidence\": 0.99}"
				}
			}]
		}`))
	}))
	defer server.Close()

	classifier := NewOpenRouterClassifierWithEndpoint("test-key", "test-model", server.URL, server.Client())
	_, err := classifier.Classify(context.Background(), ClassifyInput{
		Type:              "expense",
		Merchant:          "UNKNOWN STORE",
		AllowedCategories: []string{"Belum Dikategorikan", "Makanan & Minuman"},
	})
	if err == nil {
		t.Fatalf("expected error for category not in allowed set")
	}
}

func TestOpenRouterClassifier_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "NOT VALID JSON"
				}
			}]
		}`))
	}))
	defer server.Close()

	classifier := NewOpenRouterClassifierWithEndpoint("test-key", "test-model", server.URL, server.Client())
	_, err := classifier.Classify(context.Background(), ClassifyInput{
		Type:              "expense",
		Merchant:          "TEST",
		AllowedCategories: []string{"Belum Dikategorikan"},
	})
	if err == nil {
		t.Fatalf("expected error for malformed json completion")
	}
}

func TestOpenRouterClassifier_HTTPErrorAndRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	classifier := NewOpenRouterClassifierWithEndpoint("test-key", "test-model", server.URL, server.Client())
	_, err := classifier.Classify(context.Background(), ClassifyInput{
		Type:              "expense",
		Merchant:          "TEST",
		AllowedCategories: []string{"Belum Dikategorikan"},
	})
	if err == nil {
		t.Fatalf("expected error for 429 status")
	}
}

func TestOpenRouterClassifier_UnconfiguredKey(t *testing.T) {
	classifier := NewOpenRouterClassifier("", "test-model")
	_, err := classifier.Classify(context.Background(), ClassifyInput{
		Type:              "expense",
		Merchant:          "TEST",
		AllowedCategories: []string{"Belum Dikategorikan"},
	})
	if err == nil {
		t.Fatalf("expected error when api key is empty")
	}
}

func TestOpenRouterClassifier_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	classifier := NewOpenRouterClassifierWithEndpoint("test-key", "test-model", server.URL, server.Client())
	_, err := classifier.Classify(ctx, ClassifyInput{
		Type:              "expense",
		Merchant:          "TEST",
		AllowedCategories: []string{"Belum Dikategorikan"},
	})
	if err == nil {
		t.Fatalf("expected error on timeout")
	}
}
