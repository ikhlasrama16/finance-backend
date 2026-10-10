package parser

import (
	"regexp"
	"strings"
)

type seaBankParser struct{}

var seaAmountRE = regexp.MustCompile(`(?i)(?:sebesar|senilai)\s*(?:rp\s*)?([\d.,]+)`)
var seaReferenceRE = regexp.MustCompile(`(?i)\bref(?:erence)?\.?\s*(?:number|no\.)?`)
var incomingFromRE = regexp.MustCompile(`(?i)\bdari\b\s+(.+?)(?:\s+pada|\s+sebesar|\.|$)`)
var paymentToRE = regexp.MustCompile(`(?i)\b(?:kepada|ke)\b\s+(.+?)(?:\s+pada|\s+sebesar|\.|$)`)
var bayarInstanMerchantRE = regexp.MustCompile(`(?i)(?:bayar\s+instan[^\w]+)?pembayaran\s+([a-z0-9]+)\s+kamu\s+sebesar`)
var (
	seaTransferAmountRE = regexp.MustCompile(`(?i)(?:jumlah\s+transfer|total\s+transfer|nominal)\s*(?:rp\s*)?([\d.,]+)`)
	seaReceiptDariRE    = regexp.MustCompile(`(?i)\bdari\b\s*[:\-]?\s*(.+?)(?:\s*>\s*|\s*\r?\n|\s+seabank:|\s+ke\b|\s+jumlah\b|$)`)
	seaReceiptKeRE      = regexp.MustCompile(`(?i)\bke\b\s*[:\-]?\s*(.+?)(?:\s*>\s*|\s*\r?\n|\s+seabank:|\s+jumlah\b|\s+no\.|\s+waktu|$)`)
	seaReceiptCatatanRE = regexp.MustCompile(`(?i)\bcatatan\b\s*[:\-]?\s*([^\r\n]+?)(?:\s*\r?\n|\s+butuh\b|$)`)
)

func cleanReceiptParty(val string) string {
	val = strings.TrimSpace(val)
	val = strings.Trim(val, " \t\r\n.,;:->")
	for _, suffix := range []string{"seabank", "shopeepay", "bank jago", "kantong utama"} {
		if idx := strings.Index(strings.ToLower(val), suffix); idx > 0 {
			val = strings.TrimSpace(val[:idx])
			val = strings.Trim(val, " \t\r\n.,;:->")
		}
	}
	return val
}

func (seaBankParser) CanParse(input Input) bool {
	source := accountFromSource(input.SourceApp)
	if source == "SeaBank" {
		return true
	}
	if isOwnedAccount(source) {
		return false
	}
	text := strings.ToLower(combinedText(input))
	return strings.Contains(text, "seabank")
}

func (seaBankParser) Parse(input Input) (*Result, error) {
	text, normalized := combinedText(input), normalizedInput(input)

	// Screen receipt / Rincian Transaksi check
	if containsAny(normalized, "rincian transaksi", "jumlah transfer") || (strings.Contains(normalized, "dari") && strings.Contains(normalized, "ke") && strings.Contains(normalized, "seabank")) {
		amount := amountFromRegex(text, seaTransferAmountRE)
		if amount == 0 {
			amount = seaBankAmount(text)
		}
		if amount > 0 {
			sender := cleanReceiptParty(capture(seaReceiptDariRE, text))
			recipient := cleanReceiptParty(capture(seaReceiptKeRE, text))

			note := cleanReceiptParty(capture(seaReceiptCatatanRE, text))
			if strings.EqualFold(note, "butuh bantuan") || strings.HasPrefix(strings.ToLower(note), "butuh bantuan") {
				note = ""
			}

			// Case 1: User is recipient -> Income
			if isOwnerName(recipient) {
				desc := "Transfer Masuk SeaBank"
				if note != "" {
					desc = note
				}
				if owned := detectOwnedAccount(sender); owned != "" && owned != "SeaBank" {
					return &Result{
						Type:                   "transfer",
						Amount:                 amount,
						SourceAccountName:      owned,
						DestinationAccountName: "SeaBank",
						Description:            desc,
						ParseStatus:            "AUTO",
						Confidence:             0.98,
					}, nil
				}
				return &Result{
					Type:                   "income",
					Amount:                 amount,
					DestinationAccountName: "SeaBank",
					Merchant:               sender,
					CategoryName:           "Pemasukan",
					Description:            desc,
					ParseStatus:            "AUTO",
					Confidence:             0.95,
				}, nil
			}

			// Case 2: User is sender -> Expense (or Transfer to owned account)
			if isOwnerName(sender) {
				desc := "Transfer Keluar SeaBank"
				if note != "" {
					desc = note
				}
				if owned := detectOwnedAccount(recipient); owned != "" && owned != "SeaBank" {
					return &Result{
						Type:                   "transfer",
						Amount:                 amount,
						SourceAccountName:      "SeaBank",
						DestinationAccountName: owned,
						Description:            desc,
						ParseStatus:            "AUTO",
						Confidence:             0.98,
					}, nil
				}
				return &Result{
					Type:              "expense",
					Amount:            amount,
					SourceAccountName: "SeaBank",
					Merchant:          recipient,
					CategoryName:      "Belum Dikategorikan",
					Description:       desc,
					ParseStatus:       "AUTO",
					Confidence:        0.95,
				}, nil
			}

			// Case 3: Neither explicitly matched isOwnerName, but recipient exists
			if recipient != "" {
				desc := "Transfer SeaBank"
				if note != "" {
					desc = note
				}
				return &Result{
					Type:              "expense",
					Amount:            amount,
					SourceAccountName: "SeaBank",
					Merchant:          recipient,
					CategoryName:      "Belum Dikategorikan",
					Description:       desc,
					ParseStatus:       "AUTO",
					Confidence:        0.80,
				}, nil
			}
		}
	}

	if containsAny(normalized, "top up berhasil") && containsAny(normalized, "top up shopeepay", "shopeepay") {
		amount := seaBankAmount(text)
		return &Result{Type: "transfer", Amount: amount, SourceAccountName: "SeaBank", DestinationAccountName: "ShopeePay", Merchant: "ShopeePay", Description: "Top Up ShopeePay", ParseStatus: "AUTO", Confidence: 0.99}, nil
	}
	if containsAny(normalized, "dana masuk", "menerima dana", "transfer masuk", "uang masuk") {
		amount := seaBankAmount(text)
		source := capture(incomingFromRE, text)
		if amount == 0 {
			return nil, nil
		}
		if owned := detectOwnedAccount(source); owned != "" && owned != "SeaBank" {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: owned, DestinationAccountName: "SeaBank", ParseStatus: "AUTO", Confidence: 0.97}, nil
		}
		return &Result{Type: "income", Amount: amount, DestinationAccountName: "SeaBank", Merchant: source, CategoryName: "Pemasukan", ParseStatus: "AUTO", Confidence: 0.82}, nil
	}
	if containsAny(normalized, "realtime transfer", "transfer senilai", "melakukan transfer", "transfer virtual account", "transfer berhasil", "transfer keluar", "kirim uang") {
		amount := seaBankAmount(text)
		merchant := capture(paymentToRE, text)
		if amount == 0 {
			return nil, nil
		}
		if merchant == "" {
			return &Result{
				Type:              "expense",
				Amount:            amount,
				SourceAccountName: "SeaBank",
				CategoryName:      "Belum Dikategorikan",
				Description:       "Realtime Transfer SeaBank",
				ParseStatus:       "FAILED",
				Confidence:        0.85,
			}, nil
		}
		if owned := detectOwnedAccount(merchant); owned != "" && owned != "SeaBank" {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: "SeaBank", DestinationAccountName: owned, ParseStatus: "AUTO", Confidence: 0.98}, nil
		}
		return &Result{Type: "expense", Amount: amount, SourceAccountName: "SeaBank", Merchant: merchant, CategoryName: "Belum Dikategorikan", ParseStatus: "AUTO", Confidence: 0.90}, nil
	}
	if containsAny(normalized, "seabank bayar instan", "bayar instan") {
		amount := seaBankAmount(text)
		merchant := capture(bayarInstanMerchantRE, text)
		if merchant == "" {
			merchant = capture(paymentToRE, text)
		}
		if amount == 0 {
			return nil, nil
		}
		if owned := detectOwnedAccount(merchant); owned != "" && owned != "SeaBank" {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: "SeaBank", DestinationAccountName: owned, ParseStatus: "AUTO", Confidence: 0.98}, nil
		}
		return &Result{Type: "expense", Amount: amount, SourceAccountName: "SeaBank", Merchant: merchant, CategoryName: "Belum Dikategorikan", Description: "SeaBank Bayar Instan", ParseStatus: "AUTO", Confidence: 0.95}, nil
	}
	if containsAny(normalized, "pembayaran berhasil", "pembayaran sukses", "transaksi berhasil") {
		amount := seaBankAmount(text)
		destination := capture(paymentToRE, text)
		if destination == "" {
			destination = capture(bayarInstanMerchantRE, text)
		}
		if amount == 0 {
			return nil, nil
		}
		if owned := detectOwnedAccount(destination); owned != "" && owned != "SeaBank" {
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

func seaBankAmount(text string) int64 {
	if amount := amountFromRegex(text, seaAmountRE); amount != 0 {
		return amount
	}

	// Reference numbers may be longer than the actual amount. Never use the
	// generic largest-number fallback for messages that contain one.
	if seaReferenceRE.MatchString(strings.ToLower(text)) {
		return 0
	}

	amount, _ := extractBestAmount(text)
	return amount
}
