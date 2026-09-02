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

func (f fakeCategoryResolver) GetByID(_ context.Context, id int64) (category.Category, error) {
	for _, value := range f.categories {
		if value.ID == id {
			return value, nil
		}
	}
	return category.Category{}, errors.New("not found")
}

func (f fakeCategoryResolver) ListByType(_ context.Context, categoryType string) ([]category.Category, error) {
	var list []category.Category
	for _, cat := range f.categories {
		if cat.Type == categoryType {
			list = append(list, cat)
		}
	}
	return list, nil
}

type fakeRuleRepository struct {
	parserRules   []rule.ParserRule
	categoryRules []rule.CategoryRule
	createdRules  []rule.CategoryRule
}

func (f *fakeRuleRepository) ListActiveParserRules(context.Context) ([]rule.ParserRule, error) {
	return f.parserRules, nil
}

func (f *fakeRuleRepository) ListActiveCategoryRules(context.Context) ([]rule.CategoryRule, error) {
	return f.categoryRules, nil
}

func (f *fakeRuleRepository) CreateCategoryRule(_ context.Context, keyword string, categoryID int64, confidence float64, priority int) (rule.CategoryRule, error) {
	r := rule.CategoryRule{ID: int64(len(f.createdRules) + 1), Keyword: keyword, CategoryID: categoryID, Confidence: confidence, Priority: priority}
	f.createdRules = append(f.createdRules, r)
	f.categoryRules = append(f.categoryRules, r)
	return r, nil
}

type fakeClassifier struct {
	result *category.ClassifyResult
	err    error
	called bool
}

func (f *fakeClassifier) Classify(_ context.Context, _ category.ClassifyInput) (*category.ClassifyResult, error) {
	f.called = true
	return f.result, f.err
}

func TestDatabaseRuleParsingAndCategoryOverride(t *testing.T) {
	typeCase := "expense"
	categoryID := int64(99)
	service := &Service{ruleRepository: &fakeRuleRepository{
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
	service := &Service{ruleRepository: &fakeRuleRepository{parserRules: []rule.ParserRule{{ID: 1, Keyword: stringPointer("special notice"), Action: "IGNORE", Confidence: .8}}}}
	ignored, name, err := service.parse(context.Background(), parser.Input{SourceApp: "Any", Text: "SPECIAL NOTICE"})
	if err != nil || ignored == nil || !ignored.Ignore || ignored.ParseStatus != "IGNORED" || name != "rule" {
		t.Fatalf("unexpected ignored result: %#v, %s, %v", ignored, name, err)
	}
	fallback, name, err := service.parse(context.Background(), parser.Input{SourceApp: "SeaBank", Title: "Pembayaran QRIS", Text: "Pembayaran QRIS untuk TOKO sebesar Rp1.000"})
	if err != nil || fallback == nil || fallback.Type != "expense" || name != "parser" {
		t.Fatalf("rule miss should preserve parser fallback: %#v, %s, %v", fallback, name, err)
	}
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

func TestAIClassificationAppliesCategoryWithoutInsertingCategoryRule(t *testing.T) {
	makanan := category.Category{ID: 50, Name: "Makanan & Minuman", Type: "expense"}
	uncategorized := category.Category{ID: 21, Name: "Belum Dikategorikan", Type: "expense"}

	catResolver := fakeCategoryResolver{categories: map[string]category.Category{
		"Makanan & Minuman/expense":   makanan,
		"Belum Dikategorikan/expense": uncategorized,
	}}

	ruleRepo := &fakeRuleRepository{}
	fakeCls := &fakeClassifier{
		result: &category.ClassifyResult{
			Category:   "Makanan & Minuman",
			Confidence: 0.95,
		},
	}

	service := &Service{
		categoryRepository: catResolver,
		ruleRepository:     ruleRepo,
		classifier:         fakeCls,
	}

	parsed := &parser.Result{
		Type:              "expense",
		Amount:            16000,
		SourceAccountName: "SeaBank",
		Merchant:          "BAKSO ZAKI WONOGIRI",
		ParseStatus:       "AUTO",
	}

	if err := service.applyCategoryRule(context.Background(), parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. AI high-confidence classification applies category to current transaction
	if !fakeCls.called {
		t.Fatalf("expected classifier to be called")
	}
	if parsed.CategoryID == nil || *parsed.CategoryID != makanan.ID {
		t.Fatalf("expected category ID %d, got %#v", makanan.ID, parsed.CategoryID)
	}

	// 2. AI classification does NOT insert category_rule automatically
	if len(ruleRepo.createdRules) != 0 {
		t.Fatalf("AI classification MUST NOT insert category_rule automatically, got %d rules", len(ruleRepo.createdRules))
	}
}

func TestExistingCategoryRuleAvoidsAICall(t *testing.T) {
	makanan := category.Category{ID: 50, Name: "Makanan & Minuman", Type: "expense"}
	ruleRepo := &fakeRuleRepository{
		categoryRules: []rule.CategoryRule{
			{ID: 10, Keyword: "BAKSO ZAKI", CategoryID: makanan.ID, Confidence: 1.0, Priority: 10},
		},
	}
	fakeCls := &fakeClassifier{
		result: &category.ClassifyResult{Category: "Makanan & Minuman", Confidence: 0.95},
	}

	service := &Service{
		categoryRepository: fakeCategoryResolver{categories: map[string]category.Category{}},
		ruleRepository:     ruleRepo,
		classifier:         fakeCls,
	}

	parsed := &parser.Result{
		Type:              "expense",
		Amount:            16000,
		SourceAccountName: "SeaBank",
		Merchant:          "BAKSO ZAKI WONOGIRI",
		ParseStatus:       "AUTO",
	}

	if err := service.applyCategoryRule(context.Background(), parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3. Existing category_rule avoids AI call
	if fakeCls.called {
		t.Fatalf("existing category_rule must avoid AI call")
	}
	if parsed.CategoryID == nil || *parsed.CategoryID != makanan.ID {
		t.Fatalf("expected category ID %d from existing rule, got %#v", makanan.ID, parsed.CategoryID)
	}
}

func TestExplicitLearningAndFutureRuleUsageWithoutAI(t *testing.T) {
	makanan := category.Category{ID: 50, Name: "Makanan & Minuman", Type: "expense"}
	ruleRepo := &fakeRuleRepository{}

	// 4. Explicit LearnRule can persist merchant mapping
	learnedRule, err := ruleRepo.CreateCategoryRule(context.Background(), "BAKSO ZAKI WONOGIRI", makanan.ID, 1.0, 10)
	if err != nil || learnedRule.ID == 0 {
		t.Fatalf("failed to create explicit category rule: %v", err)
	}
	if len(ruleRepo.createdRules) != 1 {
		t.Fatalf("expected 1 created rule, got %d", len(ruleRepo.createdRules))
	}

	// 5. Future transaction uses the explicitly learned rule without AI
	fakeCls := &fakeClassifier{
		result: &category.ClassifyResult{Category: "Makanan & Minuman", Confidence: 0.95},
	}
	service := &Service{
		categoryRepository: fakeCategoryResolver{categories: map[string]category.Category{}},
		ruleRepository:     ruleRepo,
		classifier:         fakeCls,
	}

	futureTransaction := &parser.Result{
		Type:              "expense",
		Amount:            20000,
		SourceAccountName: "SeaBank",
		Merchant:          "BAKSO ZAKI WONOGIRI",
		ParseStatus:       "AUTO",
	}

	if err := service.applyCategoryRule(context.Background(), futureTransaction); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fakeCls.called {
		t.Fatalf("future transaction matching explicitly learned rule must NOT call AI")
	}
	if futureTransaction.CategoryID == nil || *futureTransaction.CategoryID != makanan.ID {
		t.Fatalf("expected category ID %d from learned rule, got %#v", makanan.ID, futureTransaction.CategoryID)
	}
}

func TestAIClassifierLowConfidenceRejected(t *testing.T) {
	makanan := category.Category{ID: 50, Name: "Makanan & Minuman", Type: "expense"}
	uncategorized := category.Category{ID: 21, Name: "Belum Dikategorikan", Type: "expense"}

	catResolver := fakeCategoryResolver{categories: map[string]category.Category{
		"Makanan & Minuman/expense":   makanan,
		"Belum Dikategorikan/expense": uncategorized,
	}}

	fakeCls := &fakeClassifier{
		result: &category.ClassifyResult{
			Category:   "Makanan & Minuman",
			Confidence: 0.40, // Below 0.85 threshold
		},
	}

	service := &Service{
		categoryRepository: catResolver,
		ruleRepository:     &fakeRuleRepository{},
		classifier:         fakeCls,
	}

	parsed := &parser.Result{
		Type:              "expense",
		Amount:            50000,
		SourceAccountName: "SeaBank",
		Merchant:          "DHEVIA LEUYS THIAQUFYAN",
		ParseStatus:       "AUTO",
	}

	if err := service.applyCategoryRule(context.Background(), parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.CategoryID != nil {
		t.Fatalf("low confidence AI prediction should not set CategoryID, got %#v", parsed.CategoryID)
	}
}

func TestAIClassifierErrorFallback(t *testing.T) {
	fakeCls := &fakeClassifier{
		err: errors.New("openrouter timeout"),
	}

	service := &Service{
		categoryRepository: fakeCategoryResolver{categories: map[string]category.Category{}},
		ruleRepository:     &fakeRuleRepository{},
		classifier:         fakeCls,
	}

	parsed := &parser.Result{
		Type:              "expense",
		Amount:            16000,
		SourceAccountName: "SeaBank",
		Merchant:          "BAKSO ZAKI WONOGIRI",
		ParseStatus:       "AUTO",
	}

	if err := service.applyCategoryRule(context.Background(), parsed); err != nil {
		t.Fatalf("AI error should fall back silently, got error: %v", err)
	}
}

func TestTransferBypassesAIClassifier(t *testing.T) {
	fakeCls := &fakeClassifier{
		result: &category.ClassifyResult{Category: "Makanan & Minuman", Confidence: 0.99},
	}
	service := &Service{classifier: fakeCls}
	parsed := &parser.Result{Type: "transfer", Amount: 50000}
	if err := service.applyCategoryRule(context.Background(), parsed); err != nil {
		t.Fatal(err)
	}
	if fakeCls.called {
		t.Fatalf("AI classifier must NOT be called for transfers")
	}
}

func TestProcessNotificationWithPromoWordingSucceeds(t *testing.T) {
	seaBank := account.Account{ID: 10, Name: "SeaBank"}
	uncategorized := category.Category{ID: 21, Name: "Belum Dikategorikan", Type: "expense"}
	service := &Service{
		accountRepository:  fakeAccountResolver{accounts: map[string]account.Account{"SeaBank": seaBank}},
		categoryRepository: fakeCategoryResolver{categories: map[string]category.Category{"Belum Dikategorikan/expense": uncategorized}},
	}
	parsed, parserName, err := service.parse(context.Background(), parser.Input{
		SourceApp: "SeaBank",
		Title:     "Pembayaran QRIS Berhasil",
		Text:      "Pembayaran QRIS untuk WARUNG KOPI sebesar Rp25.000 telah berhasil. Nikmati voucher diskon berikutnya!",
	})
	if err != nil || parsed == nil || parsed.Ignore {
		t.Fatalf("expected successful parse, got parsed=%#v, name=%s, err=%v", parsed, parserName, err)
	}
	resolved, err := service.resolve(context.Background(), Notification{ID: 99, ReceivedAt: mustTime("2026-08-16T12:00:00Z")}, parsed)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.Type != "expense" || resolved.Amount != 25000 || resolved.SourceAccountID == nil || *resolved.SourceAccountID != seaBank.ID || resolved.CategoryID == nil || *resolved.CategoryID != uncategorized.ID {
		t.Fatalf("unexpected resolved transaction: %#v", resolved)
	}
}

func mustTime(value string) (result time.Time) { result, _ = time.Parse(time.RFC3339, value); return }
