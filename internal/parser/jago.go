package parser

import (
	"regexp"
)

type jagoParser struct{}

var jagoAmountRE = regexp.MustCompile(`(?i)(?:transfer|uang|dana|sebesar|senilai|pembayaran)\s*(?:rp\s*)?([\d.,]+)`)
var jagoPaymentToRE = regexp.MustCompile(`(?i)\b(?:kepada|ke)\b\s+(.+?)(?:\s+pada|\s+sebesar|\.|\bbutuh\b|$)`)
var jagoIncomingFromRE = regexp.MustCompile(`(?i)\bdari\b\s+(.+?)(?:\s+pada|\s+sebesar|\.|\bke\b|\bbutuh\b|$)`)

func (jagoParser) CanParse(input Input) bool {
	return accountFromSource(input.SourceApp) == "Bank Jago"
}

func jagoExtractAmount(text string) int64 {
	if m := jagoAmountRE.FindStringSubmatch(text); len(m) >= 2 {
		if n, ok := parseRupiah(m[1]); ok && n > 0 {
			return n
		}
	}
	amount, _ := extractBestAmount(text)
	return amount
}

func (jagoParser) Parse(input Input) (*Result, error) {
	text, normalized := combinedText(input), normalizedInput(input)

	// Incoming funds / transfer masuk
	if containsAny(normalized, "uang masuk", "dana masuk", "transfer masuk", "menerima dana", "menerima uang", "menerima transfer", "saldo bertambah") {
		amount := jagoExtractAmount(text)
		if amount == 0 {
			return nil, nil
		}
		sender := capture(jagoIncomingFromRE, text)
		if owned := detectOwnedAccount(sender); owned != "" && owned != "Bank Jago" {
			return &Result{
				Type:                   "transfer",
				Amount:                 amount,
				SourceAccountName:      owned,
				DestinationAccountName: "Bank Jago",
				ParseStatus:            "AUTO",
				Confidence:             0.98,
			}, nil
		}
		return &Result{
			Type:                   "income",
			Amount:                 amount,
			DestinationAccountName: "Bank Jago",
			Merchant:               sender,
			CategoryName:           "Pemasukan",
			ParseStatus:            "AUTO",
			Confidence:             0.90,
		}, nil
	}

	// Outgoing transfer / transfer keluar
	if containsAny(normalized, "melakukan transfer", "transfer berhasil", "transfer keluar", "transfer senilai", "transfer sebesar", "kirim uang") {
		amount := jagoExtractAmount(text)
		if amount == 0 {
			return nil, nil
		}
		recipient := capture(jagoPaymentToRE, text)
		if recipient == "" {
			return &Result{
				Type:              "expense",
				Amount:            amount,
				SourceAccountName: "Bank Jago",
				CategoryName:      "Belum Dikategorikan",
				Description:       "Transfer Bank Jago",
				ParseStatus:       "AUTO",
				Confidence:        0.85,
			}, nil
		}
		if owned := detectOwnedAccount(recipient); owned != "" && owned != "Bank Jago" {
			return &Result{
				Type:                   "transfer",
				Amount:                 amount,
				SourceAccountName:      "Bank Jago",
				DestinationAccountName: owned,
				ParseStatus:            "AUTO",
				Confidence:             0.98,
			}, nil
		}
		return &Result{
			Type:              "expense",
			Amount:            amount,
			SourceAccountName: "Bank Jago",
			Merchant:          recipient,
			CategoryName:      "Belum Dikategorikan",
			ParseStatus:       "AUTO",
			Confidence:        0.90,
		}, nil
	}

	// Payments / pembelian / transaksi lain
	if containsAny(normalized, "pembayaran berhasil", "pembayaran sukses", "transaksi berhasil", "pembayaran sebesar", "pembelian berhasil") {
		amount := jagoExtractAmount(text)
		if amount == 0 {
			return nil, nil
		}
		recipient := capture(jagoPaymentToRE, text)
		if owned := detectOwnedAccount(recipient); owned != "" && owned != "Bank Jago" {
			return &Result{
				Type:                   "transfer",
				Amount:                 amount,
				SourceAccountName:      "Bank Jago",
				DestinationAccountName: owned,
				ParseStatus:            "AUTO",
				Confidence:             0.98,
			}, nil
		}
		return &Result{
			Type:              "expense",
			Amount:            amount,
			SourceAccountName: "Bank Jago",
			Merchant:          recipient,
			CategoryName:      "Belum Dikategorikan",
			Description:       "Pembayaran Bank Jago",
			ParseStatus:       "AUTO",
			Confidence:        0.90,
		}, nil
	}

	return nil, nil
}
