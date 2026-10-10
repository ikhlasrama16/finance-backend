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
}
