package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"finance-monitor/backend/internal/account"
	"finance-monitor/backend/internal/category"
	"finance-monitor/backend/internal/parser"
	"finance-monitor/backend/internal/rule"
)

type fakeAccountResolver struct{ accounts map[string]account.Account }

func (f fakeAccountResolver) GetByName(_ context.Context, name string) (account.Account, error) {
	value, ok := f.accounts[name]
	if !ok {
		return account.Account{}, errors.New("not found")
	}
	return value, nil
}

type fakeCategoryResolver struct{ categories map[string]category.Category }

func (f fakeCategoryResolver) GetByNameAndType(_ context.Context, name, categoryType string) (category.Category, error) {
	value, ok := f.categories[name+"/"+categoryType]
	if !ok {
		return category.Category{}, errors.New("not found")
	}
	return value, nil
}

type fakeRuleRepository struct {
	parserRules   []rule.ParserRule
	categoryRules []rule.CategoryRule
}

func (f fakeRuleRepository) ListActiveParserRules(context.Context) ([]rule.ParserRule, error) {
	return f.parserRules, nil
}
func (f fakeRuleRepository) ListActiveCategoryRules(context.Context) ([]rule.CategoryRule, error) {
	return f.categoryRules, nil
}

func TestDatabaseRuleParsingAndCategoryOverride(t *testing.T) {
	typeCase := "expense"
	categoryID := int64(99)
	service := &Service{ruleRepository: fakeRuleRepository{
		parserRules:   []rule.ParserRule{{ID: 1, SourceApp: stringPointer("SeaBank"), Keyword: stringPointer("special"), Action: "PARSE", TransactionType: &typeCase, CategoryID: &categoryID, Merchant: stringPointer("MCDONALDS"), Confidence: .91}},
		categoryRules: []rule.CategoryRule{{ID: 2, Keyword: "mcdonald", CategoryID: 100, Confidence: .95}},
	}}
	parsed, parserName, err := service.parse(context.Background(), parser.Input{SourceApp: "SeaBank", Title: "Special payment", Text: "sebesar Rp12.000"})
	if err != nil || parsed == nil || parserName != "rule" {
		t.Fatalf("unexpected parse: %#v, %s, %v", parsed, parserName, err)
	}
	if parsed.Type != "expense" || parsed.Amount != 12000 || parsed.Merchant != "MCDONALDS" || parsed.CategoryID == nil || *parsed.CategoryID != 99 {
		t.Fatalf("unexpected rule result: %#v", parsed)
	}
	if err := service.applyCategoryRule(context.Background(), parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.CategoryID == nil || *parsed.CategoryID != 100 {
		t.Fatalf("category rule did not override: %#v", parsed)
	}
}

func TestDatabaseIgnoreRuleAndFallback(t *testing.T) {
	service := &Service{ruleRepository: fakeRuleRepository{parserRules: []rule.ParserRule{{ID: 1, Keyword: stringPointer("special notice"), Action: "IGNORE", Confidence: .8}}}}
	ignored, name, err := service.parse(context.Background(), parser.Input{SourceApp: "Any", Text: "SPECIAL NOTICE"})
	if err != nil || ignored == nil || !ignored.Ignore || ignored.ParseStatus != "IGNORED" || name != "rule" {
		t.Fatalf("unexpected ignored result: %#v, %s, %v", ignored, name, err)
	}
	fallback, name, err := service.parse(context.Background(), parser.Input{SourceApp: "SeaBank", Title: "Pembayaran QRIS", Text: "Pembayaran QRIS untuk TOKO sebesar Rp1.000"})
	if err != nil || fallback == nil || fallback.Type != "expense" || name != "parser" {
		t.Fatalf("rule miss should preserve parser fallback: %#v, %s, %v", fallback, name, err)
	}
}
func (f fakeCategoryResolver) GetByID(_ context.Context, id int64) (category.Category, error) {
	for _, value := range f.categories {
		if value.ID == id {
			return value, nil
		}
	}
	return category.Category{}, errors.New("not found")
}

func TestResolveParsedNotificationCases(t *testing.T) {
	seaBank, shopeePay := account.Account{ID: 10, Name: "SeaBank"}, account.Account{ID: 11, Name: "ShopeePay"}
	pemasukan := category.Category{ID: 20, Name: "Pemasukan", Type: "income"}
	uncategorized := category.Category{ID: 21, Name: "Belum Dikategorikan", Type: "expense"}
	service := &Service{
		accountRepository:  fakeAccountResolver{accounts: map[string]account.Account{"SeaBank": seaBank, "ShopeePay": shopeePay}},
		categoryRepository: fakeCategoryResolver{categories: map[string]category.Category{"Pemasukan/income": pemasukan, "Belum Dikategorikan/expense": uncategorized}},
	}
	raw := Notification{ID: 5, ReceivedAt: mustTime("2026-08-14T10:00:00Z")}
	tests := []struct {
		name                string
		result              parser.Result
		typ                 string
		source, destination string
		categoryID          int64
	}{
		{"qris expense", parser.Result{Type: "expense", Amount: 25000, SourceAccountName: "SeaBank", CategoryName: "Belum Dikategorikan", ParseStatus: "AUTO"}, "expense", "10", "", uncategorized.ID},
		{"income", parser.Result{Type: "income", Amount: 3000000, DestinationAccountName: "SeaBank", CategoryName: "Pemasukan", ParseStatus: "AUTO"}, "income", "", "10", pemasukan.ID},
		{"one transfer", parser.Result{Type: "transfer", Amount: 100000, SourceAccountName: "SeaBank", DestinationAccountName: "ShopeePay", ParseStatus: "AUTO"}, "transfer", "10", "11", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.resolve(context.Background(), raw, &tt.result)
			if err != nil {
				t.Fatal(err)
			}
			if got.Type != tt.typ || (tt.categoryID == 0 && got.CategoryID != nil) || (tt.categoryID != 0 && (got.CategoryID == nil || *got.CategoryID != tt.categoryID)) {
				t.Fatalf("unexpected %#v", got)
			}
			if tt.source == "" && got.SourceAccountID != nil || tt.source != "" && got.SourceAccountID == nil {
				t.Fatalf("source account mismatch: %#v", got)
			}
			if tt.destination == "" && got.DestinationAccountID != nil || tt.destination != "" && got.DestinationAccountID == nil {
				t.Fatalf("destination account mismatch: %#v", got)
			}
		})
	}
}

func TestResolveQRISWithRequiredReferences(t *testing.T) {
	seaBank := account.Account{ID: 10, Name: "SeaBank"}
	uncategorized := category.Category{ID: 21, Name: "Belum Dikategorikan", Type: "expense"}
	service := &Service{
		accountRepository:  fakeAccountResolver{accounts: map[string]account.Account{"SeaBank": seaBank}},
		categoryRepository: fakeCategoryResolver{categories: map[string]category.Category{"Belum Dikategorikan/expense": uncategorized}},
	}
	parsed, err := parser.Parse(parser.Input{
		SourceApp: "SeaBank",
		Title:     "Pembayaran QRIS Berhasil",
		Text:      "Pembayaran QRIS untuk WARUNG TEST sebesar Rp10000 berhasil",
	})
	if err != nil || parsed == nil {
		t.Fatalf("parse QRIS: result=%#v err=%v", parsed, err)
	}

	got, err := service.resolve(context.Background(), Notification{ID: 5, ReceivedAt: mustTime("2026-08-15T05:19:58Z")}, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "expense" || got.Amount != 10000 || got.SourceAccountID == nil || *got.SourceAccountID != seaBank.ID || got.DestinationAccountID != nil || got.CategoryID == nil || *got.CategoryID != uncategorized.ID || got.Merchant == nil || *got.Merchant != "WARUNG TEST" || got.ParseStatus != "AUTO" || got.Confidence != .99 || got.RawNotificationID != 5 {
		t.Fatalf("unexpected parsed transaction input: %#v", got)
	}
}

func TestResolveFailsWithoutRequiredAccountOrCategory(t *testing.T) {
	service := &Service{accountRepository: fakeAccountResolver{accounts: map[string]account.Account{}}, categoryRepository: fakeCategoryResolver{categories: map[string]category.Category{}}}
	raw := Notification{ID: 1, ReceivedAt: mustTime("2026-08-14T10:00:00Z")}
	_, err := service.resolve(context.Background(), raw, &parser.Result{Type: "expense", Amount: 1000, SourceAccountName: "SeaBank", CategoryName: "Belum Dikategorikan"})
	if err == nil || err.Error() != "account not found: SeaBank" {
		t.Fatalf("got %v, want account not found diagnostic", err)
	}
}

func TestResolveExpenseDefaultsMissingCategory(t *testing.T) {
	seaBank := account.Account{ID: 10, Name: "SeaBank"}
	uncategorized := category.Category{ID: 21, Name: "Belum Dikategorikan", Type: "expense"}
	service := &Service{
		accountRepository:  fakeAccountResolver{accounts: map[string]account.Account{"SeaBank": seaBank}},
		categoryRepository: fakeCategoryResolver{categories: map[string]category.Category{"Belum Dikategorikan/expense": uncategorized}},
	}
	got, err := service.resolve(context.Background(), Notification{ID: 1, ReceivedAt: mustTime("2026-08-15T12:00:00Z")}, &parser.Result{Type: "expense", Amount: 26000, SourceAccountName: "SeaBank"})
	if err != nil || got.CategoryID == nil || *got.CategoryID != uncategorized.ID {
		t.Fatalf("unexpected result %#v, err %v", got, err)
	}
}

func TestShopeePayTopUpIsSupportingNotification(t *testing.T) {
	parsed, err := parser.Parse(parser.Input{SourceApp: "ShopeePay", Title: "Isi Saldo Berhasil", Text: "Pengisian saldo sebesar Rp10.000 telah ditambahkan ke ShopeePay-mu."})
	if err != nil || parsed == nil {
		t.Fatalf("parse: %#v, %v", parsed, err)
	}
	if !parsed.Ignore || parsed.Type != "" || parsed.SourceAccountName != "" || parsed.DestinationAccountName != "" {
		t.Fatalf("supporting notification must not create a transfer: %#v", parsed)
	}
}

func mustTime(value string) (result time.Time) { result, _ = time.Parse(time.RFC3339, value); return }
