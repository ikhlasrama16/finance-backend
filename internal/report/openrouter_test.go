package report

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenRouterClientGenerate(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"success", http.StatusOK, `{"choices":[{"message":{"content":"Ringkasan laporan"}}]}`, false},
		{"rate limited", http.StatusTooManyRequests, `{}`, true},
		{"server error", http.StatusInternalServerError, `{}`, true},
		{"malformed response", http.StatusOK, `not-json`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("Authorization = %q", got)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			client := NewOpenRouterClientWithEndpoint("test-key", "openrouter/free", server.URL, server.Client())
			content, err := client.Generate(context.Background(), "summary")
			if tt.wantErr {
				if !errors.Is(err, ErrAIUnavailable) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || content != "Ringkasan laporan" {
				t.Fatalf("content=%q err=%v", content, err)
			}
		})
	}
}

func TestOpenRouterClientContextCancellation(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	client := NewOpenRouterClientWithEndpoint("test-key", "openrouter/free", server.URL, server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := client.Generate(ctx, "summary")
	close(release)
	server.Close()
	if !errors.Is(err, ErrAIUnavailable) || !strings.Contains(err.Error(), "AI report is unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenRouterClientTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	client := NewOpenRouterClientWithEndpoint("test-key", "openrouter/free", server.URL, &http.Client{Timeout: 20 * time.Millisecond})
	_, err := client.Generate(context.Background(), "summary")
	close(release)
	server.Close()
	if !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenRouterClientRequiresKey(t *testing.T) {
	client := NewOpenRouterClient("", "")
	if _, err := client.Generate(context.Background(), "summary"); !errors.Is(err, ErrAIUnconfigured) {
		t.Fatalf("error = %v", err)
	}
}
