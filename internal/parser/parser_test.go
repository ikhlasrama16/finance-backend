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
		{"cashback transaction", Input{Title: "Pembayaran berhasil dapat cashback"}, false},
		{"top up transaction", Input{Title: "Top Up Berhasil"}, false},
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

func TestAccountMappingAndDetection(t *testing.T) {
	for source, want := range map[string]string{"seabank": "SeaBank", "shopeepay": "ShopeePay", "jago": "Bank Jago", "livin": "Mandiri", "brimo": "BRI", "shopee": "Shopee"} {
		if got := accountFromSource(source); got != want {
			t.Errorf("%s: got %s, want %s", source, got, want)
		}
	}
	for text, want := range map[string]string{"transfer ke Bank Jago": "Bank Jago", "dari ShopeePay": "ShopeePay", "dari BRI": "BRI", "ke SeaBank": "SeaBank"} {
		if got := detectOwnedAccount(text); got != want {
			t.Errorf("%s: got %s, want %s", text, got, want)
		}
	}
}

func TestQRIS(t *testing.T) {
	got, err := Parse(Input{SourceApp: "SeaBank", Title: "Pembayaran QRIS Berhasil", Text: "Pembayaran QRIS untuk WARUNG MAKMUR sebesar Rp25.000 berhasil"})
	if err != nil || got == nil {
		t.Fatalf("got %#v, err %v", got, err)
	}
	if got.Type != "expense" || got.Amount != 25000 || got.SourceAccountName != "SeaBank" || got.Merchant != "WARUNG MAKMUR" || got.ParseStatus != "AUTO" || got.Confidence != .99 {
		t.Fatalf("unexpected result %#v", got)
	}
}

func TestSeaBankCases(t *testing.T) {
	tests := []struct {
		name                          string
		input                         Input
		typ                           string
		amount                        int64
		source, destination, merchant string
	}{
		{"income", Input{SourceApp: "SeaBank", Title: "Dana Masuk", Text: "Anda menerima dana sebesar Rp3.000.000 dari PT CONTOH pada 14 Agustus 2026"}, "income", 3000000, "", "SeaBank", "PT CONTOH"},
		{"transfer in", Input{SourceApp: "SeaBank", Title: "Dana Masuk", Text: "Anda menerima dana sebesar Rp100.000 dari Bank Jago pada 14 Agustus 2026"}, "transfer", 100000, "Bank Jago", "SeaBank", ""},
		{"transfer out", Input{SourceApp: "SeaBank", Title: "Pembayaran Berhasil", Text: "Pembayaran sebesar Rp100.000 kepada ShopeePay pada 14 Agustus 2026"}, "transfer", 100000, "SeaBank", "ShopeePay", ""},
		{"expense", Input{SourceApp: "SeaBank", Title: "Pembayaran Berhasil", Text: "Pembayaran sebesar Rp45.000 kepada WARUNG SEDERHANA pada 14 Agustus 2026"}, "expense", 45000, "SeaBank", "", "WARUNG SEDERHANA"},
		{"shopeepay top up", Input{SourceApp: "SeaBank", Title: "Top Up Berhasil", Text: "Top up ShopeePay sebesar Rp75.000"}, "transfer", 75000, "SeaBank", "ShopeePay", "ShopeePay"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil || got == nil {
				t.Fatalf("got %#v, err %v", got, err)
			}
			if got.Type != tt.typ || got.Amount != tt.amount || got.SourceAccountName != tt.source || got.DestinationAccountName != tt.destination || got.Merchant != tt.merchant {
				t.Fatalf("unexpected result %#v", got)
			}
		})
	}
}

func TestShopeePayTopUpDoesNotInventSource(t *testing.T) {
	got, err := Parse(Input{SourceApp: "ShopeePay", Title: "Isi Saldo Berhasil", Text: "Pengisian saldo sebesar Rp50.000 berhasil"})
	if err != nil || got == nil || got.Type != "transfer" || got.Amount != 50000 || got.SourceAccountName != "" || got.DestinationAccountName != "ShopeePay" {
		t.Fatalf("unexpected result %#v, err %v", got, err)
	}
}

func TestGenericCases(t *testing.T) {
	tests := []struct {
		name    string
		input   Input
		typ     string
		status  string
		wantNil bool
	}{
		{"expense", Input{SourceApp: "SeaBank", Title: "Pembayaran berhasil", Text: "Pembayaran kepada TOKO sebesar Rp20.000"}, "expense", "AUTO", false},
		{"income", Input{SourceApp: "SeaBank", Title: "Gaji", Text: "Gaji sebesar Rp5.000.000"}, "income", "AUTO", false},
		{"incoming", Input{SourceApp: "SeaBank", Title: "Transfer masuk", Text: "Dana dari PT X sebesar Rp100.000"}, "income", "AUTO", false},
		{"outgoing transfer", Input{SourceApp: "SeaBank", Title: "Transfer berhasil", Text: "Transfer ke ShopeePay sebesar Rp100.000"}, "transfer", "AUTO", false},
		{"unknown", Input{SourceApp: "SeaBank", Text: "Catatan sebesar Rp1.000"}, "expense", "NEEDS_REVIEW", false},
		{"no amount", Input{SourceApp: "SeaBank", Text: "Pembayaran berhasil"}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil || (got == nil) != tt.wantNil {
				t.Fatalf("got %#v, err %v", got, err)
			}
			if !tt.wantNil && (got.Type != tt.typ || got.ParseStatus != tt.status) {
				t.Fatalf("unexpected result %#v", got)
			}
		})
	}
}
