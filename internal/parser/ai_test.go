package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouterParser_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"type\": \"expense\", \"amount\": 9300000, \"merchant\": \"MUHAMAD FATONY ANWAR\", \"source_account\": \"Bank Jago\", \"destination_account\": \"BCA\", \"description\": \"Transfer\", \"category\": \"Belanja\", \"confidence\": 0.95}"
				}
			}]
		}`))
	}))
	defer server.Close()

	parser := NewOpenRouterParserWithEndpoint("test-key", "test-model", server.URL, server.Client())
	res, err := parser.Parse(context.Background(), Input{
		SourceApp: "iOS Auto Log",
		Title:     "Transaksi Berhasil",
		Text:      "Rincian Transaksi\nMUHAMAD FATONY ANWAR\nBCA 1040668272\n-Rp9.300.000\nDari Kantong Utama",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "expense" {
		t.Fatalf("expected expense, got %s", res.Type)
	}
	if res.Amount != 9300000 {
		t.Fatalf("expected 9300000, got %d", res.Amount)
	}
	if res.Merchant != "MUHAMAD FATONY ANWAR" {
		t.Fatalf("expected MUHAMAD FATONY ANWAR, got %s", res.Merchant)
	}
	if res.SourceAccountName != "Bank Jago" {
		t.Fatalf("expected Bank Jago, got %s", res.SourceAccountName)
	}
}

func TestOpenRouterParser_Income(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"type\": \"income\", \"amount\": 4100000, \"merchant\": \"MUHAMMAD IKHLAS RAMADHAN\", \"source_account\": \"SeaBank\", \"destination_account\": \"Bank Jago\", \"description\": \"Transfer masuk\", \"category\": \"Pemasukan\", \"confidence\": 0.95}"
				}
			}]
		}`))
	}))
	defer server.Close()

	parser := NewOpenRouterParserWithEndpoint("test-key", "test-model", server.URL, server.Client())
	res, err := parser.Parse(context.Background(), Input{
		SourceApp: "iOS Auto Log",
		Title:     "Transaksi Berhasil",
		Text:      "MUHAMMAD IKHLAS RAMADHAN\nSeabank 901647608380\n+Rp4.100.000\nKe Kantong Utama\nKategori Uang masuk",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != "income" {
		t.Fatalf("expected income, got %s", res.Type)
	}
	if res.Amount != 4100000 {
		t.Fatalf("expected 4100000, got %d", res.Amount)
	}
	if res.Merchant != "MUHAMMAD IKHLAS RAMADHAN" {
		t.Fatalf("expected MUHAMMAD IKHLAS RAMADHAN, got %s", res.Merchant)
	}
	if res.DestinationAccountName != "Bank Jago" {
		t.Fatalf("expected Bank Jago, got %s", res.DestinationAccountName)
	}
}

func TestGenericParser_BankJagoScreen(t *testing.T) {
	p := genericParser{}
	// Test Uang Keluar with Kantong Utama and -Rp
	res, err := p.Parse(Input{
		SourceApp: "iOS Auto Log",
		Text:      "Rincian Transaksi\nMUHAMAD FATONY ANWAR\nBCA 1040668272\n-Rp9.300.000\nDari Kantong Utama\nKategori Uang keluar",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result from generic parser")
	}
	if res.Type != "expense" {
		t.Fatalf("expected expense, got %s", res.Type)
	}
	if res.Amount != 9300000 {
		t.Fatalf("expected 9300000, got %d", res.Amount)
	}
	if res.SourceAccountName != "Bank Jago" {
		t.Fatalf("expected Bank Jago, got %s", res.SourceAccountName)
	}
	if res.Merchant != "MUHAMAD FATONY ANWAR" {
		t.Fatalf("expected MUHAMAD FATONY ANWAR, got %s", res.Merchant)
	}
}
