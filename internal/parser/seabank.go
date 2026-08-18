package parser

import "regexp"

type seaBankParser struct{}

var seaAmountRE = regexp.MustCompile(`(?i)(?:sebesar|senilai)\s*rp\s*([\d.,]+)`)
var incomingFromRE = regexp.MustCompile(`(?i)dari\s+(.+?)\s+pada\s+`)
var paymentToRE = regexp.MustCompile(`(?i)kepada\s+(.+?)\s+pada\s+`)
var bayarInstanMerchantRE = regexp.MustCompile(`(?i)pembayaran\s+([a-z0-9\s]+?)\s+(?:kamu\s+)?sebesar`)

func (seaBankParser) CanParse(input Input) bool {
	return accountFromSource(input.SourceApp) == "SeaBank"
}
func (seaBankParser) Parse(input Input) (*Result, error) {
	text, normalized := combinedText(input), normalizedInput(input)
	if containsAny(normalized, "top up berhasil") && containsAny(input.Text, "top up shopeepay") {
		return &Result{Type: "transfer", Amount: amountFromRegex(text, seaAmountRE), SourceAccountName: "SeaBank", DestinationAccountName: "ShopeePay", Merchant: "ShopeePay", Description: "Top Up ShopeePay", ParseStatus: "AUTO", Confidence: 0.99}, nil
	}
	if containsAny(normalized, "dana masuk") {
		amount := amountFromRegex(text, seaAmountRE)
		source := capture(incomingFromRE, text)
		if amount == 0 {
			return nil, nil
		}
		if owned := detectOwnedAccount(source); owned != "" {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: owned, DestinationAccountName: "SeaBank", ParseStatus: "AUTO", Confidence: 0.97}, nil
		}
		return &Result{Type: "income", Amount: amount, DestinationAccountName: "SeaBank", Merchant: source, CategoryName: "Pemasukan", ParseStatus: "AUTO", Confidence: 0.82}, nil
	}
	if containsAny(normalized, "realtime transfer") {
		amount := amountFromRegex(text, seaAmountRE)
		merchant := capture(paymentToRE, text)
		if amount == 0 {
			return nil, nil
		}
		if merchant == "" {
			return &Result{
				Type:              "expense",
				Amount:            amount,
				SourceAccountName: "SeaBank",
				ParseStatus:       "FAILED",
			}, nil
		}
		if owned := detectOwnedAccount(merchant); owned != "" {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: "SeaBank", DestinationAccountName: owned, ParseStatus: "AUTO", Confidence: 0.98}, nil
		}
		return &Result{Type: "expense", Amount: amount, SourceAccountName: "SeaBank", Merchant: merchant, CategoryName: "Belum Dikategorikan", ParseStatus: "AUTO", Confidence: 0.90}, nil
	}
	if containsAny(normalized, "seabank bayar instan", "bayar instan") {
		amount := amountFromRegex(text, seaAmountRE)
		merchant := capture(bayarInstanMerchantRE, text)
		if merchant == "" {
			merchant = capture(paymentToRE, text)
		}
		if amount == 0 {
			return nil, nil
		}
		if owned := detectOwnedAccount(merchant); owned != "" {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: "SeaBank", DestinationAccountName: owned, ParseStatus: "AUTO", Confidence: 0.98}, nil
		}
		return &Result{Type: "expense", Amount: amount, SourceAccountName: "SeaBank", Merchant: merchant, CategoryName: "Belum Dikategorikan", Description: "SeaBank Bayar Instan", ParseStatus: "AUTO", Confidence: 0.95}, nil
	}
	if containsAny(normalized, "pembayaran berhasil") {
		amount := amountFromRegex(text, seaAmountRE)
		destination := capture(paymentToRE, text)
		if destination == "" {
			destination = capture(bayarInstanMerchantRE, text)
		}
		if amount == 0 {
			return nil, nil
		}
		if owned := detectOwnedAccount(destination); owned != "" {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: "SeaBank", DestinationAccountName: owned, ParseStatus: "AUTO", Confidence: 0.98}, nil
		}
		return &Result{Type: "expense", Amount: amount, SourceAccountName: "SeaBank", Merchant: destination, CategoryName: "Belum Dikategorikan", ParseStatus: "AUTO", Confidence: 0.85}, nil
	}
	return nil, nil
}

func capture(re *regexp.Regexp, text string) string {
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return cleanCapture(m[1])
}
func amountFromRegex(text string, re *regexp.Regexp) int64 {
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	n, _ := parseRupiah(m[1])
	return n
}
