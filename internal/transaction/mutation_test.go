package transaction

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"finance-monitor/backend/internal/category"
	"finance-monitor/backend/internal/rule"

	"github.com/jackc/pgx/v5"
)

type fakeMutationRepository struct {
	transactions map[int64]Transaction
	updated      Transaction
	deletedID    int64
	rawDetached  bool
}

func (r *fakeMutationRepository) List(context.Context) ([]Transaction, error) { return nil, nil }
func (r *fakeMutationRepository) Create(context.Context, Transaction) (Transaction, error) {
	return Transaction{}, nil
}
func (r *fakeMutationRepository) GetByID(_ context.Context, id int64) (Transaction, error) {
	transaction, found := r.transactions[id]
	if !found {
		return Transaction{}, pgx.ErrNoRows
	}
	return transaction, nil
}
func (r *fakeMutationRepository) Update(_ context.Context, transaction Transaction) (Transaction, error) {
	r.updated = transaction
	r.transactions[transaction.ID] = transaction
	return transaction, nil
}
func (r *fakeMutationRepository) Delete(_ context.Context, id int64) error {
	transaction, found := r.transactions[id]
	if !found {
		return pgx.ErrNoRows
	}
	r.deletedID = id
	r.rawDetached = transaction.RawNotificationID != nil
	delete(r.transactions, id)
	return nil
}

type fakeCategoryRepository map[int64]category.Category

func (r fakeCategoryRepository) GetByID(_ context.Context, id int64) (category.Category, error) {
	value, found := r[id]
	if !found {
		return category.Category{}, pgx.ErrNoRows
	}
	return value, nil
}

func pointer[T any](value T) *T { return &value }

func TestUpdateTransactionCategoryAndAuditMetadata(t *testing.T) {
	foodID, rawID := int64(20), int64(99)
	repository := &fakeMutationRepository{transactions: map[int64]Transaction{
		1: {ID: 1, Type: "expense", Amount: 37000, SourceAccountID: pointer(int64(3)), CategoryID: pointer(int64(10)), Source: "notification", RawNotificationID: &rawID},
	}}
	service := NewService(repository, fakeCategoryRepository{foodID: {ID: foodID, Name: "Makanan & Minuman", Type: "expense"}})
	updated, err := service.Update(context.Background(), 1, UpdateInput{CategoryID: &foodID})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Amount != 37000 || updated.Type != "expense" || updated.Source != "notification" || updated.RawNotificationID == nil || *updated.RawNotificationID != rawID {
		t.Fatalf("audit or ledger fields changed: %#v", updated)
	}
	if updated.CategoryID == nil || *updated.CategoryID != foodID || repository.updated.CategoryID == nil || *repository.updated.CategoryID != foodID {
		t.Fatalf("category was not corrected: %#v", updated)
	}
}

func TestUpdateTransactionValidation(t *testing.T) {
	expenseID, incomeID := int64(10), int64(11)
	repository := &fakeMutationRepository{transactions: map[int64]Transaction{
		1: {ID: 1, Type: "expense", Source: "notification", CategoryID: &expenseID},
		2: {ID: 2, Type: "transfer", Source: "manual"},
	}}
	service := NewService(repository, fakeCategoryRepository{
		expenseID: {ID: expenseID, Type: "expense"}, incomeID: {ID: incomeID, Type: "income"},
	})
	tests := []struct {
		name  string
		id    int64
		input UpdateInput
		want  error
	}{
		{"category type mismatch", 1, UpdateInput{CategoryID: &incomeID}, ErrCategoryTypeMismatch},
		{"transfer cannot receive category", 2, UpdateInput{CategoryID: &expenseID}, ErrCategoryNotAllowed},
		{"nonexistent transaction", 99, UpdateInput{CategoryID: &expenseID}, ErrTransactionNotFound},
		{"missing category", 1, UpdateInput{CategoryID: pointer(int64(999))}, ErrCategoryNotFound},
		{"empty patch", 1, UpdateInput{}, ErrNoUpdateFields},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.Update(context.Background(), tt.id, tt.input); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDeleteTransactionSafety(t *testing.T) {
	rawID := int64(77)
	repository := &fakeMutationRepository{transactions: map[int64]Transaction{
		1: {ID: 1, Type: "expense", Amount: 37000, Source: "manual"},
		2: {ID: 2, Type: "expense", Amount: 37000, Source: "notification", RawNotificationID: &rawID},
		3: {ID: 3, Type: "income", Amount: 100000, Source: "reconcile"},
	}}
	service := NewService(repository, fakeCategoryRepository{})
	result, err := service.Delete(context.Background(), 1)
	if err != nil || result.ID != 1 || repository.deletedID != 1 {
		t.Fatalf("manual delete result=%#v err=%v", result, err)
	}
	if _, err := service.Delete(context.Background(), 2); err != nil || !repository.rawDetached {
		t.Fatalf("parser transaction delete did not preserve raw notification: %v", err)
	}
	if _, err := service.Delete(context.Background(), 99); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("nonexistent delete error = %v", err)
	}
	if _, err := service.Delete(context.Background(), 3); !errors.Is(err, ErrReconciliationDelete) {
		t.Fatalf("reconciliation delete error = %v", err)
	}
}

func TestDeletedTransactionsChangeDerivedBalances(t *testing.T) {
	seaBank := int64(500000)
	expense := Transaction{Type: "expense", Amount: 37000, SourceAccountID: pointer(int64(1))}
	income := Transaction{Type: "income", Amount: 50000, DestinationAccountID: pointer(int64(1))}
	if got := derivedBalance(seaBank, 1, []Transaction{expense}); got != 463000 {
		t.Fatalf("balance with expense = %d", got)
	}
	if got := derivedBalance(seaBank, 1, nil); got != 500000 {
		t.Fatalf("balance after deleted expense = %d", got)
	}
	if got := derivedBalance(seaBank, 1, []Transaction{income}); got != 550000 {
		t.Fatalf("balance with income = %d", got)
	}
	if got := derivedBalance(seaBank, 1, nil); got != 500000 {
		t.Fatalf("balance after deleted income = %d", got)
	}
}

func TestDeletedTransferReversesBothDerivedBalances(t *testing.T) {
	transfer := Transaction{Type: "transfer", Amount: 10000, SourceAccountID: pointer(int64(1)), DestinationAccountID: pointer(int64(2))}
	if got := derivedBalance(50000, 1, []Transaction{transfer}); got != 40000 {
		t.Fatalf("source balance with transfer = %d", got)
	}
	if got := derivedBalance(20000, 2, []Transaction{transfer}); got != 30000 {
		t.Fatalf("destination balance with transfer = %d", got)
	}
	if source, destination := derivedBalance(50000, 1, nil), derivedBalance(20000, 2, nil); source != 50000 || destination != 20000 {
		t.Fatalf("balances after deleted transfer = %d, %d", source, destination)
	}
}

func TestDeleteRepositoryDetachesRawNotificationBeforeDelete(t *testing.T) {
	if !strings.Contains(detachRawNotificationSQL, "UPDATE raw_notifications") || !strings.Contains(detachRawNotificationSQL, "transaction_id = NULL") || !strings.Contains(detachRawNotificationSQL, "status = 'detached'") {
		t.Fatal("raw notification must be detached and preserved")
	}
	if !strings.Contains(deleteTransactionSQL, "DELETE FROM transactions") {
		t.Fatal("transaction delete query is missing")
	}
}

func TestTransactionMutationHandlerContract(t *testing.T) {
	categoryID := int64(10)
	repository := &fakeMutationRepository{transactions: map[int64]Transaction{1: {ID: 1, Type: "expense", Source: "notification"}}}
	handler := NewHandler(NewService(repository, fakeCategoryRepository{categoryID: {ID: categoryID, Type: "expense"}}))

	patch := httptest.NewRequest(http.MethodPatch, "/api/v1/transactions/1", bytes.NewBufferString(`{"category_id":10}`))
	patch.SetPathValue("id", "1")
	patchResponse := httptest.NewRecorder()
	handler.Update(patchResponse, patch)
	if patchResponse.Code != http.StatusOK || !bytes.Contains(patchResponse.Body.Bytes(), []byte(`"success":true`)) {
		t.Fatalf("patch response: %d %s", patchResponse.Code, patchResponse.Body.String())
	}

	invalid := httptest.NewRequest(http.MethodPatch, "/api/v1/transactions/1", bytes.NewBufferString(`{"amount":1}`))
	invalid.SetPathValue("id", "1")
	invalidResponse := httptest.NewRecorder()
	handler.Update(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid response: %d %s", invalidResponse.Code, invalidResponse.Body.String())
	}
}

func derivedBalance(openingBalance, accountID int64, transactions []Transaction) int64 {
	balance := openingBalance
	for _, transaction := range transactions {
		switch transaction.Type {
		case "income":
			if transaction.DestinationAccountID != nil && *transaction.DestinationAccountID == accountID {
				balance += transaction.Amount
			}
		case "expense":
			if transaction.SourceAccountID != nil && *transaction.SourceAccountID == accountID {
				balance -= transaction.Amount
			}
		case "transfer":
			if transaction.SourceAccountID != nil && *transaction.SourceAccountID == accountID {
				balance -= transaction.Amount
			}
			if transaction.DestinationAccountID != nil && *transaction.DestinationAccountID == accountID {
				balance += transaction.Amount
			}
		}
	}
	return balance
}

func TestUpdateTrimsEditableText(t *testing.T) {
	repository := &fakeMutationRepository{transactions: map[int64]Transaction{1: {ID: 1, Type: "expense", Source: "manual"}}}
	service := NewService(repository, fakeCategoryRepository{})
	merchant, description := "  WARUNG  ", "   "
	updated, err := service.Update(context.Background(), 1, UpdateInput{Merchant: &merchant, Description: &description})
	if err != nil || updated.Merchant == nil || *updated.Merchant != "WARUNG" || updated.Description != nil {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
}

type fakeRuleRepo struct {
	createdMerchant string
	createdCatID    int64
}

func (f *fakeRuleRepo) CreateCategoryRule(_ context.Context, keyword string, categoryID int64, confidence float64, priority int) (rule.CategoryRule, error) {
	f.createdMerchant = keyword
	f.createdCatID = categoryID
	return rule.CategoryRule{ID: 1, Keyword: keyword, CategoryID: categoryID, Confidence: confidence, Priority: priority}, nil
}

func TestExplicitLearnRuleInUpdate(t *testing.T) {
	categoryID := int64(10)
	merchant := "WARUNG TEST"
	repository := &fakeMutationRepository{transactions: map[int64]Transaction{
		1: {ID: 1, Type: "expense", Source: "notification", Merchant: &merchant},
	}}
	ruleRepo := &fakeRuleRepo{}
	service := NewService(repository, fakeCategoryRepository{categoryID: {ID: categoryID, Type: "expense"}}).WithRuleRepository(ruleRepo)

	learn := true
	updated, err := service.Update(context.Background(), 1, UpdateInput{CategoryID: &categoryID, LearnRule: &learn})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CategoryID == nil || *updated.CategoryID != categoryID {
		t.Fatalf("expected category_id %d", categoryID)
	}
	if ruleRepo.createdMerchant != "WARUNG TEST" || ruleRepo.createdCatID != categoryID {
		t.Fatalf("expected rule creation for WARUNG TEST, got %#v", ruleRepo)
	}
}
