package parser

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var moneyRE = regexp.MustCompile(`(?i)(?:rp\s*)?\d[\d.]*(?:,\d+)?`)

func parseRupiah(value string) (int64, bool) {
	s := strings.TrimSpace(strings.ToLower(value))
	s = strings.TrimSpace(strings.TrimPrefix(s, "rp"))
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ",") {
		parts := strings.SplitN(s, ",", 2)
		whole := strings.ReplaceAll(parts[0], ".", "")
		fraction := parts[1]
		if whole == "" {
			whole = "0"
		}
		f, err := strconv.ParseFloat(whole+"."+fraction, 64)
		if err != nil {
			return 0, false
		}
		return int64(math.Floor(f + 0.5)), true
	}
	s = strings.ReplaceAll(s, ".", "")
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

func extractBestAmount(value string) (int64, bool) {
	best, found := int64(0), false
	for _, match := range moneyRE.FindAllString(value, -1) {
		digits := strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(match), "rp")), " ", "")
		if !strings.Contains(match, "rp") && !strings.Contains(match, "Rp") && !strings.Contains(match, "RP") && !strings.Contains(match, "rP") && !strings.Contains(digits, ".") && !strings.Contains(digits, ",") {
			continue
		}
		n, ok := parseRupiah(digits)
		if ok && (!found || n > best) {
			best, found = n, true
		}
	}
	return best, found
}

func ExtractBestAmount(value string) (int64, bool) { return extractBestAmount(value) }

func extractAmountAfter(value, marker string) (int64, bool) {
	idx := strings.Index(strings.ToLower(value), strings.ToLower(marker))
	if idx < 0 {
		return 0, false
	}
	return extractBestAmount(value[idx+len(marker):])
}
