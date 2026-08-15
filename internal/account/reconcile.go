package account

type reconciliationAdjustment struct {
	TransactionType      string
	CategoryType         string
	Amount               int64
	SourceAccountID      *int64
	DestinationAccountID *int64
	Description          string
	ParseStatus          string
	Source               string
	Confidence           *float64
	RawNotificationID    *int64
}

func newReconciliationAdjustment(accountID, difference int64, description string) reconciliationAdjustment {
	adjustment := reconciliationAdjustment{
		TransactionType: "income",
		CategoryType:    "income",
		Amount:          difference,
		Description:     description,
		ParseStatus:     "MANUAL",
		Source:          "reconcile",
	}
	if difference < 0 {
		adjustment.TransactionType = "expense"
		adjustment.CategoryType = "expense"
		adjustment.Amount = -difference
		adjustment.SourceAccountID = &accountID
		return adjustment
	}
	adjustment.DestinationAccountID = &accountID
	return adjustment
}
