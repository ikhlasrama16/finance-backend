package parser

import "regexp"

var qrisAmountRE = regexp.MustCompile(`(?i)sebesar\s*(?:rp\s*)?([\d.,]+)`)
var qrisMerchantRE = regexp.MustCompile(`(?i)qris\s+untuk\s+(.+?)\s+sebesar`)

type qrisParser struct{}

func (qrisParser) CanParse(input Input) bool {
	return containsAny(combinedText(input), "pembayaran qris")
}
func (p qrisParser) Parse(input Input) (*Result, error) {
	text := combinedText(input)
	amountMatch := qrisAmountRE.FindStringSubmatch(text)
	if len(amountMatch) < 2 {
		return nil, nil
	}
	amount, ok := parseRupiah(amountMatch[1])
	if !ok {
		return nil, nil
	}
	merchant := ""
	if match := qrisMerchantRE.FindStringSubmatch(text); len(match) > 1 {
		merchant = cleanCapture(match[1])
	}
	r := &Result{Type: "expense", Amount: amount, SourceAccountName: accountFromSource(input.SourceApp), Merchant: merchant, CategoryName: "Belum Dikategorikan", ParseStatus: "AUTO", Confidence: 0.90}
	if merchant != "" {
		r.Confidence = 0.99
		r.Description = "Pembayaran QRIS " + merchant
	} else {
		r.Description = "Pembayaran QRIS"
	}
	return r, nil
}
