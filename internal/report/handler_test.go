package report

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlerMonthlySuccessAndInvalidPeriod(t *testing.T) {
	repository := &memoryRepository{records: map[string][]TransactionRecord{
		"2026-08-01": {{Type: "expense", Amount: 100, Source: "manual", CategoryName: "Food"}},
		"2026-07-01": {},
	}, cache: make(map[string]CacheEntry)}
	handler := NewHandler(NewServiceWithClock(repository, &fakeGenerator{err: ErrAIUnavailable}, func() time.Time {
		return time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	}))

	valid := httptest.NewRecorder()
	handler.Create(valid, httptest.NewRequest(http.MethodPost, "/api/v1/reports/ai", bytes.NewBufferString(`{"period":"monthly"}`)))
	if valid.Code != http.StatusOK || !bytes.Contains(valid.Body.Bytes(), []byte(`"status":"unavailable"`)) || !bytes.Contains(valid.Body.Bytes(), []byte(`"expense":100`)) {
		t.Fatalf("valid response: %d %s", valid.Code, valid.Body.String())
	}
	invalid := httptest.NewRecorder()
	handler.Create(invalid, httptest.NewRequest(http.MethodPost, "/api/v1/reports/ai", bytes.NewBufferString(`{"period":"yearly"}`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid response: %d %s", invalid.Code, invalid.Body.String())
	}
}
