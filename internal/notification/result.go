package notification

type IngestionResult struct {
	RawNotificationID int64  `json:"raw_notification_id"`
	Status            string `json:"status"`
	TransactionID     *int64 `json:"transaction_id,omitempty"`
}
