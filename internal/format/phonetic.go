package format

import (
	"regexp"
	"strings"
	"unicode"
)

var phoneticPrefixRE = regexp.MustCompile(`(?i)^(?:ipa|phonetic|pronunciation|音标)\s*[:：]?\s*`)

// NormalizePhonetic standardizes IPA to /.../ form.
func NormalizePhonetic(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}

	p = phoneticPrefixRE.ReplaceAllString(p, "")
	p = strings.TrimSpace(p)
	p = stripRegionalLabel(p)
	p = unwrapPhonetic(p)
	if p == "" {
		return ""
	}
	if !looksLikePhonetic(p) {
		return ""
	}
	return "/" + p + "/"
}

func unwrapPhonetic(p string) string {
	p = strings.TrimSpace(p)
	for len(p) >= 2 {
		switch {
		case strings.HasPrefix(p, "/") && strings.HasSuffix(p, "/"):
			p = strings.TrimSpace(p[1 : len(p)-1])
		case strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]"):
			p = strings.TrimSpace(p[1 : len(p)-1])
		case strings.HasPrefix(p, "(") && strings.HasSuffix(p, ")"):
			p = strings.TrimSpace(p[1 : len(p)-1])
		default:
			return p
		}
	}
	return strings.TrimSpace(p)
}

func stripRegionalLabel(p string) string {
	for _, label := range []string{"BrE", "AmE", "UK", "US", "British", "American"} {
		p = strings.ReplaceAll(p, label, "")
	}
	return strings.TrimSpace(p)
}

func looksLikePhonetic(p string) bool {
	runes := []rune(p)
	if len(runes) == 0 || len(runes) > 80 {
		return false
	}
	if strings.Count(p, " ") > 3 {
		return false
	}

	ipaMarkers := "əɪʊæɔθðŋʃʒˈˌːɑɒɛɜ"
	for _, r := range runes {
		if strings.ContainsRune(ipaMarkers, r) {
			return true
		}
	}

	letters := 0
	for _, r := range runes {
		if unicode.IsLetter(r) || r == 'ˈ' || r == 'ˌ' || r == ' ' || r == '-' {
			letters++
		}
	}
	return letters > 0
}
