package parser

import (
	"regexp"
	"strings"
)

var accountMappings = []struct{ key, name string }{
	{"shopeepay", "ShopeePay"}, {"seabank", "SeaBank"}, {"jago", "Bank Jago"},
	{"livin", "Mandiri"}, {"mandiri", "Mandiri"}, {"brimo", "BRI"}, {"bri", "BRI"},
	{"flip", "Flip"}, {"tokopedia", "Tokopedia"}, {"shopee", "Shopee"},
}

func accountFromSource(source string) string {
	s := normalizeText(source)
	if s == "" {
		return "Unknown"
	}
	for _, mapping := range accountMappings {
		if strings.Contains(s, mapping.key) {
			return mapping.name
		}
	}
	return strings.TrimSpace(source)
}

func detectOwnedAccount(value string) string {
	for _, name := range []string{"ShopeePay", "SeaBank", "Bank Jago", "Mandiri", "BRI", "Flip"} {
		pattern := `(?i)(^|[^a-z])` + regexp.QuoteMeta(name) + `([^a-z]|$)`
		if regexp.MustCompile(pattern).MatchString(value) {
			return name
		}
	}
	return ""
}

func isOwnedAccount(name string) bool {
	switch name {
	case "SeaBank", "ShopeePay", "Bank Jago", "Mandiri", "BRI", "Flip":
		return true
	}
	return false
}
