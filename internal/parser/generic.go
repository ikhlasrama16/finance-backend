package parser

import (
	"regexp"
	"strings"
)

type genericParser struct{}

var genericMerchantRE = regexp.MustCompile(`(?i)\b(?:kepada|ke)\b\s+(.+?)(?:\s+pada|\s+sebesar|\.|$)`)
var genericFromRE = regexp.MustCompile(`(?i)\bdari\b\s+(.+?)(?:\s+pada|\s+sebesar|\.|$)`)
var genericAtRE = regexp.MustCompile(`(?i)\bdi\b\s+(.+?)(?:\s+sebesar|\.|$)`)
var screenReceiptRE = regexp.MustCompile(`(?i)(?:rincian\s+transaksi|bukti\s+transaksi)\s*[\r\n]+([^\r\n]+)`)
var genericIncomingFundsRE = regexp.MustCompile(`(?is)\bdana\b.{0,80}\bmasuk\b`)

func (genericParser) CanParse(input Input) bool { return true }
func (genericParser) Parse(input Input) (*Result, error) {
	text, normalized := combinedText(input), normalizedInput(input)
	amount, ok := extractBestAmount(text)
	if !ok {
		return nil, nil
	}
	source := accountFromSource(input.SourceApp)
	if !isOwnedAccount(source) {
		source = detectOwnedAccount(text)
	}
	destination := ""
	merchant := capture(genericMerchantRE, text)
	if merchant == "" {
		merchant = capture(genericAtRE, text)
	}
	if merchant == "" {
		if m := screenReceiptRE.FindStringSubmatch(text); len(m) > 1 {
			candidate := strings.TrimSpace(m[1])
			if !strings.EqualFold(candidate, "berhasil") && !strings.EqualFold(candidate, "sukses") && !strings.Contains(candidate, "Rp") {
				merchant = candidate
			}
		}
	}
	if owned := detectOwnedAccount(merchant); owned != "" {
		destination = owned
	}
	if genericIncomingFundsRE.MatchString(text) || containsAny(normalized, "transfer masuk", "menerima dana", "menerima transfer", "menerima uang", "uang masuk", "saldo bertambah", "isi saldo berhasil", "top up berhasil") || strings.Contains(text, "+Rp") || strings.Contains(text, "+ Rp") {
		from := capture(genericFromRE, text)
		if from == "" && merchant != "" {
			from = merchant
		}
		if owned := detectOwnedAccount(from); owned != "" && isOwnedAccount(source) && owned != source {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: owned, DestinationAccountName: source, ParseStatus: "AUTO", Confidence: 0.85}, nil
		}
		return &Result{Type: "income", Amount: amount, DestinationAccountName: sourceIfOwned(source), Merchant: from, CategoryName: "Pemasukan", ParseStatus: "AUTO", Confidence: 0.85}, nil
	}
	if containsAny(normalized, "transfer berhasil", "berhasil dikirim", "dana terkirim", "transfer keluar", "kirim uang berhasil", "melakukan transfer") {
		if destination != "" && isOwnedAccount(source) {
			return &Result{Type: "transfer", Amount: amount, SourceAccountName: source, DestinationAccountName: destination, ParseStatus: "AUTO", Confidence: 0.82}, nil
		}
		return &Result{Type: "expense", Amount: amount, SourceAccountName: sourceIfOwned(source), Merchant: merchant, CategoryName: "Belum Dikategorikan", ParseStatus: "NEEDS_REVIEW", Confidence: 0.82}, nil
	}
	if containsAny(normalized, "uang keluar", "pembayaran berhasil", "pembayaran sukses", "transaksi berhasil", "pembelian berhasil", "pembayaran", "qris", "pembelian", "bayar", "payment successful", "debit") || strings.Contains(text, "-Rp") || strings.Contains(text, "- Rp") {
		return &Result{Type: "expense", Amount: amount, SourceAccountName: sourceIfOwned(source), Merchant: merchant, CategoryName: "Belum Dikategorikan", ParseStatus: "AUTO", Confidence: 0.78}, nil
	}
	if containsAny(normalized, "gaji", "salary", "payroll", "bonus", "pendapatan", "cashback diterima") {
		return &Result{Type: "income", Amount: amount, DestinationAccountName: sourceIfOwned(source), Merchant: merchant, CategoryName: "Pemasukan", ParseStatus: "AUTO", Confidence: 0.80}, nil
	}
	return nil, nil
}

func sourceIfOwned(name string) string {
	if isOwnedAccount(name) {
		return name
	}
	return ""
}
