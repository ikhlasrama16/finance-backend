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
	// This notification confirms an incoming top-up but does not identify the
	// source account. It must not become a source-less or self transfer. The
	// corresponding source-account notification is authoritative when present.
	return &Result{Ignore: true, ParseStatus: "IGNORED_SUPPORTING_NOTIFICATION", Confidence: 0.99}, nil
}
