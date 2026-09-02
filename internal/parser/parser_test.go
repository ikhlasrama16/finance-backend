package parser

import "testing"

func TestNormalizeText(t *testing.T) {
	if got := normalizeText("  Pembayaran   Berhasil \n"); got != "pembayaran berhasil" {
		t.Fatalf("got %q", got)
	}
}

func TestParseRupiah(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		ok    bool
	}{
		{"Rp10.000", 10000, true}, {"Rp100.000", 100000, true}, {"Rp1.250.000", 1250000, true},
		{"10.000,50", 10001, true}, {"not money", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseRupiah(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got (%d, %v), want (%d, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPromotionPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		input   Input
		ignored bool
	}{
		{"promo", Input{Title: "Promo diskon voucher"}, true},
		{"voucher", Input{Text: "Dapatkan voucher khusus untukmu"}, true},
		{"Hadiah 1,7JT", Input{SourceApp: "ShopeePay", Title: "Hadiah 1,7JT di Hari Merdeka!", Text: "Hanya Berlaku Hari Ini! Kamu bisa dapat hadiah 1,7JT dengan kirim Saldo Kaget"}, true},
		{"E-Money promo", Input{SourceApp: "ShopeePay", Title: "Harga spesial Top Up E-Money Murah!", Text: "E-Money 20RB hanya Rp17.500. Top Up sekarang"}, true},
		{"Promo Paket Data", Input{SourceApp: "ShopeePay", Text: "Promo Paket Data Rp1 khusus buat kamu"}, true},
		{"Vote Koin", Input{SourceApp: "ShopeePay", Text: "Vote, Dapat Koin GRATIS!"}, true},
		{"Total Reward", Input{SourceApp: "ShopeePay", Text: "Total Rewardmu Siap Dibuka"}, true},
		{"Judi Online Warning", Input{SourceApp: "ShopeePay", Text: "Lindungi Diri dari Judi Online"}, true},
		{"Touch ID", Input{SourceApp: "SeaBank", Title: "Touch ID Berhasil Diaktifkan"}, true},
		{"Shopee Chat", Input{SourceApp: "Shopee", Text: "Kamu mendapat chat baru"}, true},
		{"Shopee Driver", Input{SourceApp: "Shopee", Text: "Pengemudi hampir tiba!"}, true},
		{"cashback transaction", Input{Title: "Pembayaran berhasil dapat cashback", Text: "Pembayaran sebesar Rp50.000 berhasil dapat cashback"}, false},
		{"QRIS with promo", Input{SourceApp: "SeaBank", Title: "Pembayaran QRIS Berhasil", Text: "Pembayaran QRIS untuk WARUNG MAKMUR sebesar Rp25.000 telah berhasil. Dapatkan diskon khusus untukmu!"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil || (got != nil && got.Ignore) != tt.ignored {
				t.Fatalf("got %#v, err %v", got, err)
			}
		})
	}
}

func TestSeaBankRealtimeTransferClassifiesExternalAndOwnedRecipients(t *testing.T) {
	tests := []struct {
		name, title, text, typ, merchant, destination string
	}{
		{name: "external", title: "Realtime Transfer", text: "Kamu baru melakukan transfer senilai Rp26.000 kepada DHEVIA LEUYS THIAQUFYAN pada 15 Agu 2026 19:24 (WIB).", typ: "expense", merchant: "DHEVIA LEUYS THIAQUFYAN"},
		{name: "owned", title: "Realtime Transfer", text: "Kamu baru melakukan transfer senilai Rp26.000 kepada ShopeePay pada 15 Agu 2026 19:24 (WIB).", typ: "transfer", destination: "ShopeePay"},
		{name: "external without realtime title", title: "Transfer Berhasil", text: "Kamu baru melakukan transfer senilai Rp26.000 kepada DHEVIA LEUYS THIAQUFYAN pada 15 Agu 2026 19:24 (WIB).", typ: "expense", merchant: "DHEVIA LEUYS THIAQUFYAN"},
		{name: "virtual account transfer", title: "Pembayaran Berhasil", text: "Kamu telah melakukan transfer virtual account sebesar Rp10.000 kepada ShopeePay pada 16 Agu 2026 01:13 WIB", typ: "transfer", destination: "ShopeePay"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(Input{SourceApp: "SeaBank", Title: tt.title, Text: tt.text})
			if err != nil || got == nil || got.Type != tt.typ || got.Amount != 26000 && got.Amount != 10000 || got.SourceAccountName != "SeaBank" || got.Merchant != tt.merchant || got.DestinationAccountName != tt.destination {
				t.Fatalf("unexpected result %#v, err %v", got, err)
			}
			if tt.typ == "expense" && got.CategoryName != "Belum Dikategorikan" {
				t.Fatalf("missing default category: %#v", got)
			}
		})
	}
}

func TestAccountMappingAndDetection(t *testing.T) {
	for source, want := range map[string]string{"seabank": "SeaBank", "shopeepay": "ShopeePay", "jago": "Bank Jago", "livin": "Mandiri", "brimo": "BRI", "shopee": "Shopee"} {
		if got := accountFromSource(source); got != want {
			t.Errorf("%s: got %s, want %s", source, got, want)
		}
	}
	for text, want := range map[string]string{"transfer ke Bank Jago": "Bank Jago", "dari ShopeePay": "ShopeePay", "dari BRI": "BRI", "ke SeaBank": "SeaBank", "top up Flip": "Flip"} {
		if got := detectOwnedAccount(text); got != want {
			t.Errorf("%s: got %s, want %s", text, got, want)
		}
	}
}

func TestMustParseRegressionCases(t *testing.T) {
	tests := []struct {
		name        string
		input       Input
		typ         string
		amount      int64
		source      string
		destination string
		merchant    string
	}{
		{
			name:     "SeaBank QRIS BAKSO ZAKI WONOGIRI",
			input:    Input{SourceApp: "SeaBank", Title: "Pembayaran QRIS Berhasil", Text: "Pembayaran QRIS untuk BAKSO ZAKI WONOGIRI sebesar 16.000 telah berhasil."},
			typ:      "expense",
			amount:   16000,
			source:   "SeaBank",
			merchant: "BAKSO ZAKI WONOGIRI",
		},
		{
			name:     "SeaBank QRIS FIRST LAUNDRY",
			input:    Input{SourceApp: "SeaBank", Title: "Pembayaran QRIS Berhasil", Text: "Pembayaran QRIS untuk FIRST LAUNDRY sebesar 20.300 telah berhasil."},
			typ:      "expense",
			amount:   20300,
			source:   "SeaBank",
			merchant: "FIRST LAUNDRY",
		},
		{
			name:     "SeaBank Bayar Instan",
			input:    Input{SourceApp: "SeaBank", Title: "Pembayaran Berhasil", Text: "SeaBank Bayar Instan: Pembayaran Shopee kamu sebesar Rp14.343 pada 14 Agu 2026 telah berhasil."},
			typ:      "expense",
			amount:   14343,
			source:   "SeaBank",
			merchant: "Shopee",
		},
		{
			name:     "SeaBank Realtime Transfer Outgoing",
			input:    Input{SourceApp: "SeaBank", Title: "Realtime Transfer", Text: "Realtime Transfer sebesar Rp50.000 kepada JOHN DOE pada 14 Agu 2026"},
			typ:      "expense",
			amount:   50000,
			source:   "SeaBank",
			merchant: "JOHN DOE",
		},
		{
			name:        "SeaBank Incoming Funds",
			input:       Input{SourceApp: "SeaBank", Title: "Dana Masuk", Text: "Anda menerima dana sebesar Rp100.000 dari JANE DOE pada 14 Agu 2026"},
			typ:         "income",
			amount:      100000,
			destination: "SeaBank",
			merchant:    "JANE DOE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil || got == nil {
				t.Fatalf("got %#v, err %v", got, err)
			}
			if got.Type != tt.typ || got.Amount != tt.amount || got.SourceAccountName != tt.source || got.DestinationAccountName != tt.destination {
				t.Fatalf("unexpected result %#v", got)
			}
			if tt.merchant != "" && got.Merchant != tt.merchant {
				t.Fatalf("got merchant %q, want %q", got.Merchant, tt.merchant)
			}
		})
	}
}

func TestMustFailSafelyIncompleteTransfer(t *testing.T) {
	input := Input{SourceApp: "SeaBank", Title: "Realtime Transfer", Text: "Realtime Transfer sebesar Rp50.000 pada 18/08/2026"}
	got, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if got == nil || got.ParseStatus != "FAILED" {
		t.Fatalf("expected ParseStatus FAILED for incomplete transfer, got %#v", got)
	}
}

func TestShopeePaySupportingTopUpIgnored(t *testing.T) {
	got, err := Parse(Input{SourceApp: "ShopeePay", Title: "Isi Saldo Berhasil", Text: "Pengisian saldo sebesar Rp10.000 telah ditambahkan ke ShopeePay-mu. Saldo saat ini sebesar Rp10.050."})
	if err != nil || got == nil || !got.Ignore || got.ParseStatus != "IGNORED_SUPPORTING_NOTIFICATION" {
		t.Fatalf("unexpected result %#v, err %v", got, err)
	}
}

func TestShopeeMarketplaceTopUpCompletionIgnored(t *testing.T) {
	got, err := Parse(Input{SourceApp: "Shopee", Title: "Top-up Completed", Text: "Your Top up request of Rp10.000 is successful and your current balance is Rp10.050."})
	if err != nil || got == nil || !got.Ignore || got.ParseStatus != "IGNORED_SUPPORTING_NOTIFICATION" {
		t.Fatalf("unexpected result %#v, err %v", got, err)
	}
}

func TestUnknownMerchantSafeFallback(t *testing.T) {
	got, err := Parse(Input{SourceApp: "SeaBank", Title: "Pembayaran Berhasil", Text: "Pembayaran sebesar Rp35.000 telah berhasil"})
	if err != nil || got == nil {
		t.Fatalf("unexpected result %#v, err %v", got, err)
	}
	if got.Type != "expense" || got.Amount != 35000 || got.Merchant != "" || got.CategoryName != "Belum Dikategorikan" || got.ParseStatus != "AUTO" {
		t.Fatalf("unexpected result for unknown merchant: %#v", got)
	}
}

func TestGenericConservativeParsing(t *testing.T) {
	// A Rupiah amount alone MUST NEVER create a transaction
	cases := []Input{
		{SourceApp: "SeaBank", Text: "Catatan sebesar Rp1.000"},
		{SourceApp: "SeaBank", Text: "Catatan Rp100.000 untuk besok"},
		{SourceApp: "Unknown", Text: "Info tagihan Rp50.000 belum lunas"},
	}
	for _, input := range cases {
		got, err := Parse(input)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if got != nil && !got.Ignore {
			t.Fatalf("expected nil or ignored result for arbitrary text with amount, got %#v", got)
		}
	}
}
