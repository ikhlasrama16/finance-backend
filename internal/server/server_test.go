package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadinessHandler(t *testing.T) {
	tests := []struct {
		name    string
		pingErr error
		want    int
		body    string
	}{
		{"ready", nil, http.StatusOK, `{"status":"ready"}`},
		{"unavailable", errors.New("database unavailable"), http.StatusServiceUnavailable, `{"status":"not_ready"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			readinessHandler(func(context.Context) error { return tt.pingErr }).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
			if res.Code != tt.want || res.Body.String() != tt.body {
				t.Fatalf("got %d %q", res.Code, res.Body.String())
			}
		})
	}
}
