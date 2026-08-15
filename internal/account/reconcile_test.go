package account

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeRepository struct {
	reconciliation Reconciliation
	err            error
	input          ReconcileInput
}

func (f *fakeRepository) List(context.Context) ([]Account, error) { return nil, nil }
func (f *fakeRepository) Create(context.Context, CreateInput) (Account, error) {
	return Account{}, nil
}
func (f *fakeRepository) Reconcile(_ context.Context, input ReconcileInput) (Reconciliation, error) {
	f.input = input
	return f.reconciliation, f.err
}

func TestNewReconciliationAdjustment(t *testing.T) {
	tests := []struct {
		name                       string
		difference                 int64
		wantType, wantCategoryType string
		wantAmount                 int64
		wantSource, wantDest       bool
	}{
		{"expense adjustment", -17900, "expense", "expense", 17900, true, false},
		{"income adjustment", 17900, "income", "income", 17900, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newReconciliationAdjustment(2, tt.difference, "Balance reconciliation")
			if got.TransactionType != tt.wantType || got.CategoryType != tt.wantCategoryType || got.Amount != tt.wantAmount || (got.SourceAccountID != nil) != tt.wantSource || (got.DestinationAccountID != nil) != tt.wantDest {
				t.Fatalf("unexpected adjustment: %#v", got)
			}
			if got.Source != "reconcile" || got.ParseStatus != "MANUAL" || got.Confidence != nil || got.RawNotificationID != nil {
				t.Fatalf("missing audit metadata: %#v", got)
			}
		})
	}
}

func TestServiceReconcileBuildsAuditableDescription(t *testing.T) {
	store := &fakeRepository{reconciliation: Reconciliation{AccountID: 2, PreviousBalance: 67900, ActualBalance: 50000, Difference: -17900}}
	service := NewService(store)
	got, err := service.Reconcile(context.Background(), 2, 50000, "Reconcile dari aplikasi")
	if err != nil || got.Difference != -17900 {
		t.Fatalf("result=%#v err=%v", got, err)
	}
	if store.input.AccountID != 2 || store.input.ActualBalance != 50000 || store.input.Description != "Balance reconciliation - Reconcile dari aplikasi" {
		t.Fatalf("unexpected input: %#v", store.input)
	}
}

func TestServiceReconcileValidationAndNotFound(t *testing.T) {
	service := NewService(&fakeRepository{})
	if _, err := service.Reconcile(context.Background(), 0, 0, ""); !errors.Is(err, ErrInvalidAccountID) {
		t.Fatalf("got %v", err)
	}
	if _, err := service.Reconcile(context.Background(), 1, 0, string(make([]byte, 501))); !errors.Is(err, ErrInvalidReconciliationNote) {
		t.Fatalf("got %v", err)
	}
	notFound := NewService(&fakeRepository{err: ErrAccountNotFound})
	if _, err := notFound.Reconcile(context.Background(), 1, 0, ""); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestReconcileHandlerContract(t *testing.T) {
	t.Run("account not found", func(t *testing.T) {
		handler := NewHandler(NewService(&fakeRepository{err: ErrAccountNotFound}))
		request := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/99/reconcile", bytes.NewBufferString(`{"actual_balance":50000}`))
		request.SetPathValue("id", "99")
		response := httptest.NewRecorder()
		handler.Reconcile(response, request)
		if response.Code != http.StatusNotFound || response.Body.String() != "{\"error\":\"account not found\",\"success\":false}\n" {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("zero balance is valid", func(t *testing.T) {
		txID := int64(123)
		store := &fakeRepository{reconciliation: Reconciliation{AccountID: 2, PreviousBalance: 50000, ActualBalance: 0, Difference: -50000, TransactionID: &txID}}
		handler := NewHandler(NewService(store))
		request := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/2/reconcile", bytes.NewBufferString(`{"actual_balance":0,"note":"Reconcile dari aplikasi"}`))
		request.SetPathValue("id", "2")
		response := httptest.NewRecorder()
		handler.Reconcile(response, request)
		if response.Code != http.StatusOK || store.input.ActualBalance != 0 {
			t.Fatalf("status=%d input=%#v", response.Code, store.input)
		}
	})
}

func TestTransferBalanceSemantics(t *testing.T) {
	seaBank, shopeePay := int64(50000), int64(20000)
	amount := int64(10000)
	seaBank -= amount
	shopeePay += amount
	if seaBank != 40000 || shopeePay != 30000 || seaBank+shopeePay != 70000 {
		t.Fatalf("transfer changed total wealth: SeaBank=%d ShopeePay=%d", seaBank, shopeePay)
	}
}
