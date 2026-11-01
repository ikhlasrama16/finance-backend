package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireSessionProtectsAPIAndAllowsIngest(t *testing.T) {
	service := NewService(&memoryRepository{users: map[string]storedUser{}, sessions: map[string]Session{}})
	protected := RequireSession(service)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.Code)
	}
	ingest := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", nil)
	response = httptest.NewRecorder()
	protected.ServeHTTP(response, ingest)
	if response.Code != http.StatusNoContent {
		t.Fatalf("ingest status=%d", response.Code)
	}

	apiKeyProtected := RequireSessionOrAPIKey(service, "secret-key")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	bearerReq := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	bearerReq.Header.Set("Authorization", "Bearer secret-key")
	response = httptest.NewRecorder()
	apiKeyProtected.ServeHTTP(response, bearerReq)
	if response.Code != http.StatusNoContent {
		t.Fatalf("bearer status=%d", response.Code)
	}

	headerKeyReq := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	headerKeyReq.Header.Set("X-API-Key", "secret-key")
	response = httptest.NewRecorder()
	apiKeyProtected.ServeHTTP(response, headerKeyReq)
	if response.Code != http.StatusNoContent {
		t.Fatalf("x-api-key status=%d", response.Code)
	}

	invalidKeyReq := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	invalidKeyReq.Header.Set("Authorization", "Bearer wrong-key")
	response = httptest.NewRecorder()
	apiKeyProtected.ServeHTTP(response, invalidKeyReq)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid bearer status=%d", response.Code)
	}
}
