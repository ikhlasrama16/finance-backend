package rule

import "strings"

func Normalize(value string) string { return strings.Join(strings.Fields(strings.ToLower(value)), " ") }

func ParserRuleMatches(value ParserRule, sourceApp, text string) bool {
	if value.SourceApp != nil && strings.TrimSpace(*value.SourceApp) != "" && !strings.Contains(Normalize(sourceApp), Normalize(*value.SourceApp)) {
		return false
	}
	if value.Keyword != nil && strings.TrimSpace(*value.Keyword) != "" && !strings.Contains(Normalize(text), Normalize(*value.Keyword)) {
		return false
	}
	return true
}

func FirstParserRule(rules []ParserRule, sourceApp, text string) *ParserRule {
	for i := range rules {
		if ParserRuleMatches(rules[i], sourceApp, text) {
			return &rules[i]
		}
	}
	return nil
}

func FirstCategoryRule(rules []CategoryRule, haystack string) *CategoryRule {
	text := Normalize(haystack)
	for i := range rules {
		if strings.TrimSpace(rules[i].Keyword) != "" && strings.Contains(text, Normalize(rules[i].Keyword)) {
			return &rules[i]
		}
	}
	return nil
}
