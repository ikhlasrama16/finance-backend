package report

import "time"

const (
	PeriodDaily   = "daily"
	PeriodWeekly  = "weekly"
	PeriodMonthly = "monthly"
	PeriodCustom  = "custom"

	defaultModel = "openrouter/free"
)

type Request struct {
	Period    string `json:"period"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

type Period struct {
	Type          string
	Start         time.Time
	EndExclusive  time.Time
	PreviousStart time.Time
	PreviousEnd   time.Time
}

func (p Period) StartDate() string { return p.Start.Format("2006-01-02") }
func (p Period) EndDate() string {
	return p.EndExclusive.AddDate(0, 0, -1).Format("2006-01-02")
}

type TransactionRecord struct {
	Type         string
	Amount       int64
	Source       string
	Merchant     string
	CategoryName string
	OccurredAt   time.Time
	CreatedAt    time.Time
}

type Summary struct {
	Income                   int64 `json:"income"`
	Expense                  int64 `json:"expense"`
	NetCashflow              int64 `json:"net_cashflow"`
	TransactionCount         int64 `json:"transaction_count"`
	ExpenseTransactionCount  int64 `json:"expense_transaction_count"`
	TransferCount            int64 `json:"transfer_count"`
	AverageDailyExpense      int64 `json:"average_daily_expense"`
	ReconciliationAdjustment int64 `json:"reconciliation_adjustment"`
}

type CategoryTotal struct {
	Category   string  `json:"category"`
	Amount     int64   `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type MerchantTotal struct {
	Merchant         string `json:"merchant"`
	Amount           int64  `json:"amount"`
	TransactionCount int64  `json:"transaction_count"`
}

type Comparison struct {
	PreviousPeriodExpense   int64   `json:"previous_period_expense"`
	ExpenseChangeAmount     int64   `json:"expense_change_amount"`
	ExpenseChangePercentage float64 `json:"expense_change_percentage"`
}

type Statistics struct {
	Summary           Summary
	ExpenseByCategory []CategoryTotal
	TopMerchants      []MerchantTotal
}

type AIResult struct {
	Content     *string    `json:"content"`
	Status      string     `json:"status"`
	Model       string     `json:"model,omitempty"`
	GeneratedAt *time.Time `json:"generated_at,omitempty"`
}

type Response struct {
	Period            string          `json:"period"`
	StartDate         string          `json:"start_date"`
	EndDate           string          `json:"end_date"`
	Summary           Summary         `json:"summary"`
	ExpenseByCategory []CategoryTotal `json:"expense_by_category"`
	TopMerchants      []MerchantTotal `json:"top_merchants"`
	Comparison        Comparison      `json:"comparison"`
	AI                AIResult        `json:"ai"`
}

type CacheEntry struct {
	Content   string
	Model     string
	CreatedAt time.Time
}
