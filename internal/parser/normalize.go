package parser

import (
	"regexp"
	"strings"
)

var whitespaceRE = regexp.MustCompile(`\s+`)

func normalizeText(value string) string {
	return strings.TrimSpace(whitespaceRE.ReplaceAllString(strings.ToLower(value), " "))
}

func combinedText(input Input) string {
	return strings.TrimSpace(strings.Join([]string{input.Title, input.Text, input.BigText}, " "))
}
func normalizedInput(input Input) string { return normalizeText(combinedText(input)) }

func containsAny(value string, words ...string) bool {
	value = normalizeText(value)
	for _, word := range words {
		if strings.Contains(value, normalizeText(word)) {
			return true
		}
	}
	return false
}

func cleanCapture(value string) string {
	return strings.TrimSpace(strings.Trim(value, " \t\r\n.,;:-"))
}
