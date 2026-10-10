package parser

import "regexp"

type shopeeParser struct{}

var shopeeRefundRE = regexp.MustCompile(`(?i)pengembalian\s+dana\s+sebesar\s+(?:rp\s*)?([\d.,]+)\s+akan\s+dikembalikan\s+ke\s+([a-z0-9]+)`)

func (shopeeParser) CanParse(input Input) bool {
	return normalizeText(input.SourceApp) == "shopee"
}

func (shopeeParser) Parse(input Input) (*Result, error) {
	text := combinedText(input)
	normalized := normalizedInput(input)

	// Top up supporting notification
	if isShopeeTopUp(input) {
		return &Result{Ignore: true, ParseStatus: "IGNORED_SUPPORTING_NOTIFICATION", Confidence: 0.99}, nil
	}

	// SPayLater bill payment confirmation
	if containsAny(normalized, "spaylater") && containsAny(normalized, "spaylater bill payment has been received", "pembayaran tagihan spaylater kamu telah diterima") {
		return &Result{Ignore: true, ParseStatus: "IGNORED_SUPPORTING_NOTIFICATION", Confidence: 0.99}, nil
	}

	// SPayLater Bill reminder
	if containsAny(normalized, "spaylater bill", "cek tagihan spaylater") {
		return &Result{Ignore: true, ParseStatus: "IGNORED_SUPPORTING_NOTIFICATION", Confidence: 0.99}, nil
	}

	// Refund / Pengembalian Dana
	if containsAny(normalized, "pengembalian dana", "pengembalian barang") {
		m := shopeeRefundRE.FindStringSubmatch(text)
		if len(m) >= 2 {
			amount, _ := parseRupiah(m[1])
			if amount > 0 {
				destination := "ShopeePay"
				if len(m) >= 3 {
					if owned := detectOwnedAccount(m[2]); owned != "" {
						destination = owned
					}
				}
				return &Result{
					Type:                   "income",
					Amount:                 amount,
					DestinationAccountName: destination,
					Merchant:               "Shopee",
					CategoryName:           "Pemasukan",
					Description:            "Pengembalian Dana / Refund Shopee",
					ParseStatus:            "AUTO",
					Confidence:             0.95,
				}, nil
			}
		}
	}

	return nil, nil
}

func isShopeeTopUp(input Input) bool {
	title := normalizeText(input.Title)
	text := normalizedInput(input)
	if containsAny(title, "top-up completed", "top up completed") {
		return true
	}
	return containsAny(text, "top up request") && containsAny(text, "successful", "success")
}
