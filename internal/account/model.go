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
