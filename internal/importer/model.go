package importer

import "time"

type LegacyRow struct {
	ID              string
	Timestamp       string
	Account         string
	Type            string
	Amount          string
	Merchant        string
	Category        string
	Description     string
	Source          string
	ParseStatus     string
	Confidence      string
	IsSynthetic     string
	TransferGroupID string
}

type Account struct {
	ID   int64
	Name string
}

type Category struct {
	ID   int64
	Name string
	Type string
}

type Transaction struct {
	LegacyID           string
	Type               string
	Amount             int64
	SourceAccountID    *int64
	DestinationAccount *int64
	CategoryID         *int64
	Merchant           *string
	Description        *string
	ParseStatus        string
	Confidence         *float64
	OccurredAt         time.Time
}

type Unresolved struct {
	LegacyID        string
	Type            string
	Account         string
	Merchant        string
	Description     string
	TransferGroupID string
	Reason          string
}

type Summary struct {
	RowsRead                      int
	ExpenseRowsImported           int
	IncomeRowsImported            int
	TransfersCollapsed            int
	SyntheticRowsIgnored          int
	UnknownRowsSkipped            int
	MarketplaceSupportSkipped     int
	ExternalTransferInsConverted  int
	ExternalTransferOutsConverted int
	UnresolvedRows                []Unresolved
	DuplicatesSkipped             int
	Errors                        int
}

type ResolvedData struct {
	Transactions []Transaction
	Summary      Summary
}
