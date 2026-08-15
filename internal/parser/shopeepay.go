package parser

import "regexp"

type shopeePayParser struct{}

var shopeeAmountRE = regexp.MustCompile(`(?i)pengisian saldo sebesar\s*rp\s*([\d.,]+)`)

func (shopeePayParser) CanParse(input Input) bool {
	return accountFromSource(input.SourceApp) == "ShopeePay"
}
func (shopeePayParser) Parse(input Input) (*Result, error) {
	if !containsAny(input.Title, "isi saldo berhasil") {
		return nil, nil
	}
	amount := amountFromRegex(combinedText(input), shopeeAmountRE)
	if amount == 0 {
		return nil, nil
	}
	return &Result{Type: "transfer", Amount: amount, DestinationAccountName: "ShopeePay", ParseStatus: "AUTO", Confidence: 0.99}, nil
}
