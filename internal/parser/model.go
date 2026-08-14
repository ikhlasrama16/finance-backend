package parser

type Input struct {
	SourceApp string
	Title     string
	Text      string
	BigText   string
}

type Result struct {
	Type                   string
	Amount                 int64
	SourceAccountName      string
	DestinationAccountName string
	Merchant               string
	CategoryName           string
	CategoryID             *int64
	Description            string
	ParseStatus            string
	Confidence             float64
	Ignore                 bool
}

type Parser interface {
	CanParse(input Input) bool
	Parse(input Input) (*Result, error)
}
