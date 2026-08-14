package rule

type ParserRule struct {
	ID              int64
	SourceApp       *string
	Keyword         *string
	Action          string
	TransactionType *string
	CategoryID      *int64
	Merchant        *string
	Confidence      float64
	Priority        int
}

type CategoryRule struct {
	ID         int64
	Keyword    string
	CategoryID int64
	Confidence float64
	Priority   int
}
