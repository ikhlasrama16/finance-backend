package notification

import (
	"encoding/json"
	"time"
)

type Notification struct {
	ID           int64           `json:"id"`
	SourceApp    string          `json:"source_app"`
	Title        *string         `json:"title,omitempty"`
	Body         string          `json:"body"`
	ReceivedAt   time.Time       `json:"received_at"`
	Status       string          `json:"status"`
	ParserName   *string         `json:"parser_name,omitempty"`
	RawPayload   json.RawMessage `json:"raw_payload,omitempty"`
	Fingerprint  *string         `json:"fingerprint,omitempty"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

type CreateInput struct {
	SourceApp  string          `json:"source_app"`
	Title      *string         `json:"title"`
	Body       string          `json:"body"`
	ReceivedAt string          `json:"received_at"`
	RawPayload json.RawMessage `json:"raw_payload"`
}
