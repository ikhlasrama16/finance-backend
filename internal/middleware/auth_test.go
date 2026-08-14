package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuth(t *testing.T) {
	tests := []struct {
		name, authorization string
		want                int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"malformed", "Token secret", http.StatusUnauthorized},
		{"invalid", "Bearer wrong", http.StatusUnauthorized},
		{"valid", "Bearer secret", http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", nil)
			request.Header.Set("Authorization", tt.authorization)
			response := httptest.NewRecorder()
			BearerAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("got %d, want %d", response.Code, tt.want)
			}
		})
	}
}
