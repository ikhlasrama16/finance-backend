package importer

import (
	"strings"
	"unicode"
)

type ownedAlias struct {
	alias string
	name  string
}

var ownedAccountAliases = []ownedAlias{
	{alias: "shopeepay", name: "ShopeePay"},
	{alias: "bank jago", name: "Bank Jago"},
	{alias: "seabank", name: "SeaBank"},
	{alias: "mandiri", name: "Mandiri"},
	{alias: "livin", name: "Mandiri"},
	{alias: "brimo", name: "BRI"},
	{alias: "flip", name: "Flip"},
	{alias: "bri", name: "BRI"},
	{alias: "jago", name: "Bank Jago"},
}

func containsOwnedAlias(text, alias string) bool {
	textTokens := lookupTokens(text)
	aliasTokens := lookupTokens(alias)
	if len(aliasTokens) == 0 || len(aliasTokens) > len(textTokens) {
		return false
	}
	for i := 0; i <= len(textTokens)-len(aliasTokens); i++ {
		matched := true
		for j := range aliasTokens {
			if textTokens[i+j] != aliasTokens[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func containsAnyOwnedAccountAlias(text string) bool {
	for _, value := range ownedAccountAliases {
		if containsOwnedAlias(text, value.alias) {
			return true
		}
	}
	return false
}

func containsPhrase(text, phrase string) bool {
	textTokens := lookupTokens(text)
	phraseTokens := lookupTokens(phrase)
	if len(phraseTokens) == 0 || len(phraseTokens) > len(textTokens) {
		return false
	}
	for i := 0; i <= len(textTokens)-len(phraseTokens); i++ {
		matched := true
		for j := range phraseTokens {
			if textTokens[i+j] != phraseTokens[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func lookupTokens(value string) []string {
	var builder strings.Builder
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
		} else {
			builder.WriteRune(' ')
		}
	}
	return strings.Fields(builder.String())
}
