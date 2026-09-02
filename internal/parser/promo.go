package parser

var promoIndicators = []string{
	"promo",
	"diskon",
	"hadiah",
	"gratis",
	"voucher",
	"reward",
	"koin",
	"komisi",
	"khusus buat kamu",
	"khusus buatmu",
	"khusus untukmu",
	"khusus untuk kamu",
	"cobain",
	"yuk",
	"serbu",
	"murah",
	"top up sekarang",
	"harga spesial",
	"total reward",
	"vote",
	"daftar qris merchant",
	"pengingat promo",
	"bonus cashback",
	"lebih hemat",
	"dapatkan tiket",
	"penawaran",
	"buruan",
	"lindungi diri",
	"saldo kaget",
}

var nonTransactionIndicators = []string{
	"kamu mendapat chat baru",
	"chat baru",
	"pesan baru",
	"pengemudi hampir tiba",
	"driver menuju lokasi",
	"pesanan sedang diantar",
	"pesanan dalam perjalanan",
	"touch id berhasil diaktifkan",
	"fingerprint berhasil",
	"passcode diubah",
	"keamanan akun",
	"perangkat baru",
	"login berhasil",
	"fitur baru",
	"cobain fitur",
	"kini hadir",
}

var transactionIndicators = []string{
	"pembayaran qris",
	"pembayaran berhasil",
	"pembayaran sukses",
	"transaksi berhasil",
	"transfer berhasil",
	"berhasil dikirim",
	"dana terkirim",
	"kirim uang berhasil",
	"telah berhasil",
	"dana masuk",
	"transfer masuk",
	"menerima dana",
	"uang masuk",
	"saldo bertambah",
	"telah masuk",
	"top up berhasil",
	"isi saldo berhasil",
	"pembayaran sebesar",
	"transfer sebesar",
	"transfer senilai",
	"melakukan transfer",
	"seabank bayar instan",
	"payment successful",
}

func isPromotion(input Input) bool {
	text := normalizedInput(input)
	if containsAny(text, nonTransactionIndicators...) {
		return true
	}
	if containsAny(text, transactionIndicators...) {
		return false
	}
	return containsAny(text, promoIndicators...)
}

func promotionResult() *Result {
	return &Result{
		Ignore:      true,
		ParseStatus: "IGNORED_PROMO",
		Confidence:  0.99,
	}
}
