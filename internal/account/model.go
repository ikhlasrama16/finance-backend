package account

import "time"

type Account struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Provider       *string   `json:"provider,omitempty"`
	Type           string    `json:"type"`
	OpeningBalance int64     `json:"opening_balance"`
	Balance        int64     `json:"balance"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateInput struct {
	Name           string  `json:"name"`
	Provider       *string `json:"provider"`
	Type           string  `json:"type"`
	OpeningBalance int64   `json:"opening_balance"`
}

type ReconcileInput struct {
	AccountID     int64
	ActualBalance int64
	Description   string
}

type Reconciliation struct {
	AccountID       int64  `json:"account_id"`
	PreviousBalance int64  `json:"previous_balance"`
	ActualBalance   int64  `json:"actual_balance"`
	Difference      int64  `json:"difference"`
	TransactionID   *int64 `json:"transaction_id"`
}
