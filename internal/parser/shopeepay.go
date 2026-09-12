package parser

import "regexp"

type shopeePayParser struct{}

var shopeeAmountRE = regexp.MustCompile(`(?i)pengisian saldo sebesar\s*rp\s*([\d.,]+)`)
var shopeeBiFastTransferInRE = regexp.MustCompile(`(?i)(.+?)\s+mengirimkan\s+dana\s+sebesar\s+(?:rp\s*)?([\d.,]+)\s+ke\s+shopeepay`)
var shopeePaymentSuccessRE = regexp.MustCompile(`(?i)pembayaran\s+sebesar\s+(?:rp\s*)?([\d.,]+)\s+dengan\s+shopeepay\s+telah\s+berhasil`)

func (shopeePayParser) CanParse(input Input) bool {
	return accountFromSource(input.SourceApp) == "ShopeePay"
}
func (shopeePayParser) Parse(input Input) (*Result, error) {
	text := combinedText(input)
	normalized := normalizedInput(input)

	// SPayLater bill payment confirmation (supporting notification)
	if containsAny(normalized, "spaylater") && containsAny(normalized, "pembayaran tagihan spaylater kamu telah diterima", "spaylater bill payment has been received") {
		return &Result{Ignore: true, ParseStatus: "IGNORED_SUPPORTING_NOTIFICATION", Confidence: 0.99}, nil
	}

	// BI-Fast incoming transfer to ShopeePay
	if containsAny(normalized, "mengirimkan dana sebesar", "saldo shopeepay diterima") {
		m := shopeeBiFastTransferInRE.FindStringSubmatch(text)
		if len(m) >= 3 {
			sender := cleanCapture(m[1])
			amount, _ := parseRupiah(m[2])
			if amount > 0 {
				if owned := detectOwnedAccount(sender); owned != "" && owned != "ShopeePay" {
					return &Result{
						Type:                   "transfer",
						Amount:                 amount,
						SourceAccountName:      owned,
						DestinationAccountName: "ShopeePay",
						ParseStatus:            "AUTO",
						Confidence:             0.95,
					}, nil
				}
				return &Result{
					Type:                   "income",
					Amount:                 amount,
					DestinationAccountName: "ShopeePay",
					Merchant:               sender,
					CategoryName:           "Pemasukan",
					Description:            "Transfer Masuk BI-Fast " + sender,
					ParseStatus:            "AUTO",
					Confidence:             0.95,
				}, nil
			}
		}
	}

	// Direct payment from ShopeePay
	if containsAny(normalized, "dengan shopeepay telah berhasil") {
		amount := amountFromRegex(text, shopeePaymentSuccessRE)
		if amount > 0 {
			return &Result{
				Type:              "expense",
				Amount:            amount,
				SourceAccountName: "ShopeePay",
				CategoryName:      "Belum Dikategorikan",
				Description:       "Pembayaran ShopeePay",
				ParseStatus:       "AUTO",
				Confidence:        0.95,
			}, nil
		}
	}

	if !containsAny(input.Title, "isi saldo berhasil") {
		return nil, nil
	}
	amount := amountFromRegex(text, shopeeAmountRE)
	if amount == 0 {
		return nil, nil
	}
	// This notification confirms an incoming top-up but does not identify the
	// source account. It must not become a source-less or self transfer. The
	// corresponding source-account notification is authoritative when present.
	return &Result{Ignore: true, ParseStatus: "IGNORED_SUPPORTING_NOTIFICATION", Confidence: 0.99}, nil
}
