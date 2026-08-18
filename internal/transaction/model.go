package transaction

import "time"

type Transaction struct {
	ID                   int64     `json:"id"`
	Type                 string    `json:"type"`
	Amount               int64     `json:"amount"`
	SourceAccountID      *int64    `json:"source_account_id,omitempty"`
	DestinationAccountID *int64    `json:"destination_account_id,omitempty"`
	CategoryID           *int64    `json:"category_id,omitempty"`
	Description          *string   `json:"description,omitempty"`
	Merchant             *string   `json:"merchant,omitempty"`
	ParseStatus          string    `json:"parse_status"`
	Confidence           *float64  `json:"confidence,omitempty"`
	Source               string    `json:"source"`
	RawNotificationID    *int64    `json:"raw_notification_id,omitempty"`
	OccurredAt           time.Time `json:"occurred_at"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type CreateParsedInput struct {
	Type                 string
	Amount               int64
	SourceAccountID      *int64
	DestinationAccountID *int64
	CategoryID           *int64
	Description          *string
	Merchant             *string
	ParseStatus          string
	Confidence           float64
	OccurredAt           string
	RawNotificationID    int64
}

type CreateInput struct {
	Type                 string  `json:"type"`
	Amount               int64   `json:"amount"`
	SourceAccountID      *int64  `json:"source_account_id"`
	DestinationAccountID *int64  `json:"destination_account_id"`
	CategoryID           *int64  `json:"category_id"`
	Description          *string `json:"description"`
	OccurredAt           string  `json:"occurred_at"`
}

// UpdateInput deliberately allows only correction fields. Ledger structure
// and origin metadata remain immutable through the PATCH endpoint.
type UpdateInput struct {
	CategoryID  *int64  `json:"category_id"`
	Merchant    *string `json:"merchant"`
	Description *string `json:"description"`
	LearnRule   *bool   `json:"learn_rule,omitempty"`
}

type DeleteResult struct {
	ID int64 `json:"id"`
}
