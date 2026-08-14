package parser

var transactionIndicators = []string{"berhasil", "telah masuk", "menerima dana", "dana masuk", "pembayaran qris", "pembayaran sebesar", "transfer sebesar", "top up berhasil", "isi saldo berhasil"}
var promoIndicators = []string{"promo", "diskon", "bonus cashback", "lebih hemat", "dapatkan tiket", "voucher", "penawaran", "buruan", "khusus untukmu", "khusus untuk kamu"}

func isPromotion(input Input) bool {
	text := normalizedInput(input)
	if containsAny(text, transactionIndicators...) {
		return false
	}
	return containsAny(text, promoIndicators...)
}

func promotionResult() *Result { return &Result{Ignore: true, ParseStatus: "IGNORED_PROMO"} }
