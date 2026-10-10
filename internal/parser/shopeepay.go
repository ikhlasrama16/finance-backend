package parser

import (
	"regexp"
	"strings"
)

type shopeePayParser struct{}

var shopeeAmountRE = regexp.MustCompile(`(?i)pengisian saldo sebesar\s*rp\s*([\d.,]+)`)
var shopeeBiFastTransferInRE = regexp.MustCompile(`(?i)(.+?)\s+mengirimkan\s+dana\s+sebesar\s+(?:rp\s*)?([\d.,]+)\s+ke\s+shopeepay`)
var shopeePaymentSuccessRE = regexp.MustCompile(`(?i)pembayaran\s+sebesar\s+(?:rp\s*)?([\d.,]+)\s+dengan\s+shopeepay\s+telah\s+berhasil`)
var (
	shopeeKamuMembayarRE = regexp.MustCompile(`(?i)(?:kamu\s+membayar|total\s+pembayaran|jumlah\s+pembayaran)\s*(?:rp\s*)?([\d.,]+)`)
	shopeeTopAmountRE    = regexp.MustCompile(`(?i)(?:^|[\r\n\s])\-?\s*rp\s*([\d.,]+)\s*(?:[\r\n\s]|berhasil)`)
	shopeeBayarKeRE      = regexp.MustCompile(`(?i)\bbayar\s+ke\b\s*[:\-]?\s*(.+?)(?:\s*>\s*|\s*\r?\n|\s+waktu\b|\s+kamu\s+membayar|$)`)
)

func (shopeePayParser) CanParse(input Input) bool {
	source := accountFromSource(input.SourceApp)
	if source == "ShopeePay" {
		return true
	}
	if isOwnedAccount(source) {
		return false
	}
	text := strings.ToLower(combinedText(input))
	return strings.Contains(text, "shopeepay") || (strings.Contains(text, "rincian pembayaran") && (strings.Contains(text, "shopee") || strings.Contains(text, "bayar ke")))
}

func (shopeePayParser) Parse(input Input) (*Result, error) {
	text := combinedText(input)
	normalized := normalizedInput(input)

	// ShopeePay screen receipt (Rincian Pembayaran / Bayar Ke)
	if containsAny(normalized, "rincian pembayaran", "kamu membayar", "bayar ke") {
		amount := amountFromRegex(text, shopeeKamuMembayarRE)
		if amount == 0 {
			amount = amountFromRegex(text, shopeeTopAmountRE)
		}
		if amount == 0 {
			amount, _ = extractBestAmount(text)
		}
		if amount > 0 {
			rawMerchant := capture(shopeeBayarKeRE, text)
			cleanMerchant := cleanReceiptParty(rawMerchant)
			isQRIS := strings.HasPrefix(strings.ToUpper(cleanMerchant), "QRIS") || strings.Contains(strings.ToLower(normalized), "qris")
			cleanMerchant = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(cleanMerchant, "QRIS"), "qris"))
			cleanMerchant = cleanReceiptParty(cleanMerchant)

			if !isValidMerchantName(cleanMerchant) {
				cleanMerchant = ""
			}

			desc := "Pembayaran ShopeePay"
			if isQRIS {
				desc = "Pembayaran QRIS ShopeePay"
			}

			return &Result{
				Type:              "expense",
				Amount:            amount,
				SourceAccountName: "ShopeePay",
				Merchant:          cleanMerchant,
				CategoryName:      "Belum Dikategorikan",
				Description:       desc,
				ParseStatus:       "AUTO",
				Confidence:        0.95,
			}, nil
		}
	}

	// SPayLater bill payment confirmation (supporting notification)
	if containsAny(normalized, "spaylater") && containsAny(normalized, "pembayaran tagihan spaylater kamu telah diterima", "spaylater bill payment has been received") {
		return &Result{Ignore: true, ParseStatus: "IGNORED_SUPPORTING_NOTIFICATION", Confidence: 0.99}, nil
	}

	// BI-Fast incoming transfer to ShopeePay
	if containsAny(normalized, "mengirimkan dana sebesar", "saldo shopeepay diterima") {
		m := shopeeBiFastTransferInRE.FindStringSubmatch(input.Text)
		if len(m) < 3 {
			m = shopeeBiFastTransferInRE.FindStringSubmatch(text)
		}
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
