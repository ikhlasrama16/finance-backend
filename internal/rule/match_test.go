package rule

import "testing"

func stringPointer(value string) *string { return &value }

func TestFirstParserRuleMatchingAndPriorityOrder(t *testing.T) {
	rules := []ParserRule{
		{ID: 10, SourceApp: stringPointer("seabank"), Keyword: stringPointer("promo khusus")},
		{ID: 11, SourceApp: stringPointer("seabank"), Keyword: stringPointer("promo")},
	}
	matched := FirstParserRule(rules, "SeaBank Mobile", "PROMO KHUSUS untuk kamu")
	if matched == nil || matched.ID != 10 {
		t.Fatalf("unexpected match: %#v", matched)
	}
	if FirstParserRule(rules, "ShopeePay", "PROMO KHUSUS") != nil {
		t.Fatal("unrelated app matched")
	}
}

func TestParserRuleEmptyFields(t *testing.T) {
	empty := ""
	rules := []ParserRule{
		{ID: 1, SourceApp: &empty, Keyword: stringPointer("voucher")},
		{ID: 2, SourceApp: stringPointer("SeaBank"), Keyword: &empty},
	}
	if matched := FirstParserRule(rules, "Other App", "Ada VOUCHER hari ini"); matched == nil || matched.ID != 1 {
		t.Fatalf("empty source app should match: %#v", matched)
	}
	if matched := FirstParserRule(rules[1:], "SeaBank", "anything"); matched == nil || matched.ID != 2 {
		t.Fatalf("empty keyword should match: %#v", matched)
	}
}

func TestFirstCategoryRulePriorityAndEmptyKeyword(t *testing.T) {
	rules := []CategoryRule{
		{ID: 20, Keyword: "mcdonald", CategoryID: 2},
		{ID: 21, Keyword: "food", CategoryID: 3},
	}
	matched := FirstCategoryRule(rules, "MCDONALDS SeaBank")
	if matched == nil || matched.ID != 20 {
		t.Fatalf("unexpected category match: %#v", matched)
	}
	if FirstCategoryRule([]CategoryRule{{Keyword: "", CategoryID: 5}}, "anything") != nil {
		t.Fatal("empty category keyword matched")
	}
}
