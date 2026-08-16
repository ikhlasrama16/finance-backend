package report

import (
	"strings"
	"testing"
	"time"
)

func TestCalculateExcludesTransfersAndReconciliation(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, jakartaLocation)
	end := start.AddDate(0, 0, 2)
	statistics := Calculate([]TransactionRecord{
		{Type: "income", Amount: 1000, Source: "manual"},
		{Type: "expense", Amount: 300, Source: "manual", CategoryName: "Food", Merchant: "Warung A"},
		{Type: "expense", Amount: 200, Source: "manual", CategoryName: "Food", Merchant: "Warung A"},
		{Type: "expense", Amount: 100, Source: "manual", CategoryName: "Transport", Merchant: "Ojek"},
		{Type: "expense", Amount: 400, Source: "import", CategoryName: "Food", Merchant: "Import Merchant"},
		{Type: "transfer", Amount: 500, Source: "manual", CategoryName: "Must not appear", Merchant: "Must not appear"},
		{Type: "expense", Amount: 50, Source: "reconcile", CategoryName: "Food", Merchant: "Adjustment"},
	}, start, end)

	if got, want := statistics.Summary.Income, int64(1000); got != want {
		t.Fatalf("income = %d, want %d", got, want)
	}
	if got, want := statistics.Summary.Expense, int64(1000); got != want {
		t.Fatalf("expense = %d, want %d", got, want)
	}
	if statistics.Summary.NetCashflow != 0 || statistics.Summary.TransactionCount != 6 || statistics.Summary.ExpenseTransactionCount != 4 || statistics.Summary.TransferCount != 1 || statistics.Summary.AverageDailyExpense != 500 || statistics.Summary.ReconciliationAdjustment != -50 {
		t.Fatalf("unexpected summary: %+v", statistics.Summary)
	}
	if len(statistics.ExpenseByCategory) != 2 || statistics.ExpenseByCategory[0] != (CategoryTotal{Category: "Food", Amount: 900, Percentage: 90}) {
		t.Fatalf("categories = %+v", statistics.ExpenseByCategory)
	}
	if len(statistics.TopMerchants) != 3 || statistics.TopMerchants[0] != (MerchantTotal{Merchant: "Warung A", Amount: 500, TransactionCount: 2}) {
		t.Fatalf("merchants = %+v", statistics.TopMerchants)
	}
}

func TestBuildComparison(t *testing.T) {
	comparison := BuildComparison(Statistics{Summary: Summary{Expense: 120}}, Statistics{Summary: Summary{Expense: 100}})
	if comparison.PreviousPeriodExpense != 100 || comparison.ExpenseChangeAmount != 20 || comparison.ExpenseChangePercentage != 20 {
		t.Fatalf("comparison = %+v", comparison)
	}
}

func TestTopMerchantsAreOrderedAndIgnoreEmptyNames(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, jakartaLocation)
	statistics := Calculate([]TransactionRecord{
		{Type: "expense", Amount: 10, Source: "manual", Merchant: ""},
		{Type: "expense", Amount: 30, Source: "manual", Merchant: "B"},
		{Type: "expense", Amount: 40, Source: "manual", Merchant: "A"},
	}, start, start.AddDate(0, 0, 1))
	if len(statistics.TopMerchants) != 2 || statistics.TopMerchants[0].Merchant != "A" || statistics.TopMerchants[1].Merchant != "B" {
		t.Fatalf("merchants = %+v", statistics.TopMerchants)
	}
}

func TestLoadTransactionsUsesOccurredAt(t *testing.T) {
	// The SQL boundary is intentionally based on the financial event time, not ingestion time.
	if !strings.Contains(loadTransactionsSQL, "t.occurred_at >= $1") || strings.Contains(loadTransactionsSQL, "created_at >=") {
		t.Fatal("report query must use occurred_at period boundaries")
	}
}
