package importer

import (
	"strings"
	"testing"
	"time"
)

func testAccounts() []Account {
	return []Account{{ID: 1, Name: "SeaBank"}, {ID: 2, Name: "ShopeePay"}, {ID: 3, Name: "BRI"}}
}

func testCategories() []Category {
	return []Category{{ID: 10, Name: "Belum Dikategorikan", Type: "expense"}, {ID: 11, Name: "Makanan & Minuman", Type: "expense"}, {ID: 12, Name: "Pemasukan", Type: "income"}}
}

func TestNormalizeExpenseAndDefaultCategory(t *testing.T) {
	data := Normalize([]LegacyRow{{ID: "expense-1", Timestamp: "8/10/2026 17:42:48", Account: "SeaBank", Type: "EXPENSE", Amount: "25000", Merchant: "ABC", Description: "Dinner"}}, testAccounts(), testCategories())
	if len(data.Transactions) != 1 {
		t.Fatalf("transactions: %#v", data)
	}
	value := data.Transactions[0]
	if value.Type != "expense" || value.Amount != 25000 || value.SourceAccountID == nil || *value.SourceAccountID != 1 || value.CategoryID == nil || *value.CategoryID != 10 || value.LegacyID != "transaction:expense-1" {
		t.Fatalf("unexpected transaction: %#v", value)
	}
}

func TestNormalizeShopeepayVariant(t *testing.T) {
	data := Normalize([]LegacyRow{{ID: "expense-1", Timestamp: "8/10/2026 17:42:48", Account: "Shopeepay", Type: "EXPENSE", Amount: "1000", Category: "Laundry"}}, testAccounts(), append(testCategories(), Category{ID: 13, Name: "Laundry", Type: "expense"}))
	if len(data.Transactions) != 1 || data.Transactions[0].SourceAccountID == nil || *data.Transactions[0].SourceAccountID != 2 {
		t.Fatalf("Shopeepay was not normalized: %#v", data)
	}
}

func TestNormalizeTransferPairIgnoresSyntheticCounterpart(t *testing.T) {
	rows := []LegacyRow{
		{ID: "in", Timestamp: "8/10/2026 17:42:48", Account: "SeaBank", Type: "TRANSFER_IN", Amount: "10000", TransferGroupID: "group-1"},
		{ID: "out", Timestamp: "8/10/2026 17:42:49", Account: "Shopeepay", Type: "TRANSFER_OUT", Amount: "10000", TransferGroupID: "group-1", IsSynthetic: "TRUE"},
	}
	data := Normalize(rows, testAccounts(), testCategories())
	if len(data.Transactions) != 1 || len(data.Summary.UnresolvedRows) != 0 || data.Summary.SyntheticRowsIgnored != 1 {
		t.Fatalf("unexpected synthetic transfer result: %#v", data)
	}
	value := data.Transactions[0]
	if value.Type != "transfer" || value.SourceAccountID == nil || *value.SourceAccountID != 2 || value.DestinationAccount == nil || *value.DestinationAccount != 1 || value.CategoryID != nil || value.LegacyID != "transfer:group-1" {
		t.Fatalf("unexpected collapsed transfer: %#v", value)
	}
}

func TestNormalizeSingleTransferAndUnresolvedTransfer(t *testing.T) {
	rows := []LegacyRow{
		{ID: "single", Timestamp: "8/10/2026 17:42:48", Account: "SeaBank", Type: "TRANSFER_OUT", Amount: "10000", Merchant: "to ShopeePay"},
		{ID: "unresolved", Timestamp: "8/10/2026 17:42:48", Account: "SeaBank", Type: "TRANSFER_OUT", Amount: "10000", Merchant: "DHEVIA"},
	}
	data := Normalize(rows, testAccounts(), testCategories())
	if len(data.Transactions) != 2 || len(data.Summary.UnresolvedRows) != 0 {
		t.Fatalf("unexpected single transfer result: %#v", data)
	}
	if data.Transactions[1].Type != "expense" || data.Transactions[1].CategoryID == nil {
		t.Fatalf("external outgoing transfer was not converted to expense: %#v", data.Transactions[1])
	}
}

func TestNormalizeTransferInferenceAliasesAndDescription(t *testing.T) {
	tests := []struct {
		name, account, typ, merchant, description, source, destination string
	}{
		{name: "transfer out merchant", account: "SeaBank", typ: "TRANSFER_OUT", merchant: "ShopeePay", source: "SeaBank", destination: "ShopeePay"},
		{name: "transfer in merchant", account: "Shopeepay", typ: "TRANSFER_IN", merchant: "SeaBank", source: "SeaBank", destination: "ShopeePay"},
		{name: "description alias", account: "Livin", typ: "TRANSFER_OUT", description: "Transfer ke Bank Jago", source: "Mandiri", destination: "Bank Jago"},
		{name: "brimo alias", account: "BRImo", typ: "TRANSFER_OUT", description: "ke Flip", source: "BRI", destination: "Flip"},
	}
	accounts := append(testAccounts(), Account{ID: 4, Name: "Bank Jago"}, Account{ID: 5, Name: "Mandiri"}, Account{ID: 6, Name: "Flip"})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := Normalize([]LegacyRow{{ID: tt.name, Timestamp: "8/10/2026 17:42:48", Account: tt.account, Type: tt.typ, Amount: "10000", Merchant: tt.merchant, Description: tt.description}}, accounts, testCategories())
			if len(data.Transactions) != 1 {
				t.Fatalf("expected one transaction: %#v", data)
			}
			value := data.Transactions[0]
			if value.SourceAccountID == nil || value.DestinationAccount == nil || accountName(accounts, *value.SourceAccountID) != tt.source || accountName(accounts, *value.DestinationAccount) != tt.destination {
				t.Fatalf("unexpected endpoints: %#v", value)
			}
		})
	}
}

func accountName(accounts []Account, id int64) string {
	for _, account := range accounts {
		if account.ID == id {
			return account.Name
		}
	}
	return ""
}

func TestNormalizeAmbiguousMerchantAndShopeeRemainUnresolved(t *testing.T) {
	data := Normalize([]LegacyRow{
		{ID: "ambiguous", Timestamp: "8/10/2026 17:42:48", Account: "Mandiri", Type: "TRANSFER_IN", Amount: "10000", Merchant: "rek"},
		{ID: "shopee", Timestamp: "8/10/2026 17:42:48", Account: "Shopee", Type: "TRANSFER_IN", Amount: "10000", Merchant: "DHEVIA"},
	}, append(testAccounts(), Account{ID: 5, Name: "Mandiri"}), testCategories())
	if len(data.Transactions) != 0 || len(data.Summary.UnresolvedRows) != 2 {
		t.Fatalf("unexpected unresolved handling: %#v", data)
	}
}

func TestNormalizeExternalTransferInsBecomeIncome(t *testing.T) {
	rows := []LegacyRow{
		{ID: "sea-income", Timestamp: "8/10/2026 17:42:48", Account: "SeaBank", Type: "TRANSFER_IN", Amount: "10000", Merchant: "DHEVIA LEUYS THIAQUFYAN"},
		{ID: "shopee-income", Timestamp: "8/10/2026 17:42:48", Account: "ShopeePay", Type: "TRANSFER_IN", Amount: "10000", Merchant: "DHEVIA LEUYS THIAQUFYAN"},
	}
	data := Normalize(rows, testAccounts(), testCategories())
	if len(data.Transactions) != 2 || data.Summary.ExternalTransferInsConverted != 2 {
		t.Fatalf("external incoming transfers were not converted: %#v", data)
	}
	for _, value := range data.Transactions {
		if value.Type != "income" || value.SourceAccountID != nil || value.DestinationAccount == nil || value.CategoryID == nil {
			t.Fatalf("unexpected external income: %#v", value)
		}
	}
}

func TestNormalizeShopeePayTopUpAndMarketplaceSupport(t *testing.T) {
	rows := []LegacyRow{
		{ID: "topup", Timestamp: "8/10/2026 17:42:48", Account: "ShopeePay", Type: "TRANSFER_IN", Amount: "10000", Description: "Isi saldo ShopeePay"},
		{ID: "support", Timestamp: "8/10/2026 17:42:48", Account: "Shopee", Type: "TRANSFER_IN", Merchant: "ShopeePay-mu", Description: "Isi Saldo Berhasil"},
	}
	data := Normalize(rows, testAccounts(), testCategories())
	if len(data.Transactions) != 0 || len(data.Summary.UnresolvedRows) != 1 || data.Summary.UnresolvedRows[0].Reason != "source account unknown for ShopeePay top-up" || data.Summary.MarketplaceSupportSkipped != 1 {
		t.Fatalf("unexpected top-up/support handling: %#v", data)
	}
}

func TestNormalizeGroupedExternalIncomeAndMarketplaceSupport(t *testing.T) {
	rows := []LegacyRow{
		{ID: "external", Timestamp: "8/10/2026 17:42:48", Account: "SeaBank", Type: "TRANSFER_IN", Amount: "10000", Merchant: "DHEVIA LEUYS THIAQUFYAN", Description: "Transfer Masuk", TransferGroupID: "external-group"},
		{ID: "support", Timestamp: "8/10/2026 17:42:48", Account: "Shopee", Type: "TRANSFER_IN", Amount: "10000", Merchant: "ShopeePay-mu", Description: "Isi Saldo Berhasil", TransferGroupID: "support-group"},
		{ID: "synthetic", Timestamp: "8/10/2026 17:42:49", Account: "ShopeePay", Type: "TRANSFER_OUT", Amount: "10000", Merchant: "Shopee", Description: "Auto counterpart ke Shopee", TransferGroupID: "support-group", IsSynthetic: "TRUE"},
	}
	data := Normalize(rows, testAccounts(), testCategories())
	if len(data.Transactions) != 1 || data.Transactions[0].Type != "income" || data.Summary.ExternalTransferInsConverted != 1 || data.Summary.MarketplaceSupportSkipped != 1 || data.Summary.SyntheticRowsIgnored != 1 {
		t.Fatalf("unexpected grouped handling: %#v", data)
	}
}

func TestNormalizeUnknownAndTimestamp(t *testing.T) {
	data := Normalize([]LegacyRow{{ID: "unknown", Timestamp: "8/10/2026 17:42:48", Account: "SeaBank", Type: "UNKNOWN", Amount: "1000"}}, testAccounts(), testCategories())
	if len(data.Transactions) != 0 || data.Summary.UnknownRowsSkipped != 1 {
		t.Fatalf("unknown was imported: %#v", data)
	}
	value, err := ParseTimestamp("8/9/2026 14:26:29")
	if err != nil || !value.Equal(time.Date(2026, 8, 9, 7, 26, 29, 0, time.UTC)) {
		t.Fatalf("unexpected Jakarta timestamp: %v, %v", value, err)
	}
}

func TestReadCSVAndDryRunNormalizationArePure(t *testing.T) {
	rows, err := ReadCSV(strings.NewReader("id,timestamp,account,type,amount,merchant,category,description,balance,source,raw_id,parse_status,confidence,is_synthetic,transfer_group_id\n1,8/10/2026 17:42:48,SeaBank,EXPENSE,1000,,,test,,import,,MANUAL,,FALSE,\n"))
	if err != nil || len(rows) != 1 {
		t.Fatalf("read csv: %#v, %v", rows, err)
	}
	data := Normalize(rows, testAccounts(), testCategories())
	if len(data.Transactions) != 1 {
		t.Fatalf("dry-run normalization failed: %#v", data)
	}
}
