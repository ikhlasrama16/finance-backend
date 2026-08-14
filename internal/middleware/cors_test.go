package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSPreflight(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/notifications", nil)
	req.Header.Set("Origin", "https://app.example")
	res := httptest.NewRecorder()
	CORS([]string{"https://app.example"}, false)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(res, req)
	if res.Code != http.StatusNoContent || res.Header().Get("Access-Control-Allow-Origin") != "https://app.example" || res.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Fatalf("unexpected CORS response: %d %#v", res.Code, res.Header())
	}
}
