package notification

type IngestionResult struct {
	RawNotificationID int64  `json:"raw_notification_id"`
	Status            string `json:"status"`
	TransactionID     *int64 `json:"transaction_id,omitempty"`
	Reason            string `json:"reason,omitempty"`
	Amount            int64  `json:"amount,omitempty"`
	Type              string `json:"type,omitempty"`
	Merchant          string `json:"merchant,omitempty"`
	Account           string `json:"account,omitempty"`
	Description       string `json:"description,omitempty"`
}
