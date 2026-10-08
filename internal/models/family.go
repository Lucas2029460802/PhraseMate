package models

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxForms   = 4
	maxPhrases = 4
)

// NormalizeForms keeps a short list of derivations in other parts of speech.
func NormalizeForms(term string, in []RelatedForm) []RelatedForm {
	term = strings.TrimSpace(term)
	out := make([]RelatedForm, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, f := range in {
		word := strings.TrimSpace(f.Word)
		if word == "" {
			word = strings.TrimSpace(f.Term)
		}
		if !usableLabel(word) || utf8.RuneCountInString(word) > 40 {
			continue
		}
		if term != "" && strings.EqualFold(word, term) {
			continue
		}
		key := strings.ToLower(word)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		zh := strings.TrimSpace(f.MeaningZH)
		if zh == "" {
			zh = strings.TrimSpace(f.Meaning)
		}
		out = append(out, RelatedForm{
			Word:      word,
			POS:       CleanPOS(firstText(f.POS, f.PartOfSpeech)),
			MeaningZH: clipRunes(zh, 40),
		})
		if len(out) >= maxForms {
			break
		}
	}
	return out
}

// NormalizePhrases keeps a short list of collocations.
func NormalizePhrases(term string, in []Phrase) []Phrase {
	term = strings.TrimSpace(term)
	out := make([]Phrase, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, p := range in {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			text = strings.TrimSpace(p.Alt)
		}
		if !usableLabel(text) || utf8.RuneCountInString(text) > 80 {
			continue
		}
		if term != "" && strings.EqualFold(text, term) {
			continue
		}
		key := strings.ToLower(text)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		zh := strings.TrimSpace(p.MeaningZH)
		if zh == "" {
			zh = strings.TrimSpace(p.Meaning)
		}
		out = append(out, Phrase{
			Text:      text,
			MeaningZH: clipRunes(zh, 40),
		})
		if len(out) >= maxPhrases {
			break
		}
	}
	return out
}

// CleanPOS maps common part-of-speech labels to the notebook's short form.
func CleanPOS(pos string) string {
	pos = strings.TrimSpace(pos)
	if pos == "" {
		return ""
	}
	key := strings.ToLower(pos)
	key = strings.Trim(key, ".")
	key = strings.ReplaceAll(key, " ", "")
	switch key {
	case "n", "noun", "名词":
		return "n."
	case "v", "verb", "动词":
		return "v."
	case "adj", "adjective", "形容词":
		return "adj."
	case "adv", "adverb", "副词":
		return "adv."
	case "prep", "preposition", "介词":
		return "prep."
	case "conj", "conjunction", "连词":
		return "conj."
	case "pron", "pronoun", "代词":
		return "pron."
	case "phrase", "短语":
		return "phrase"
	default:
		return pos
	}
}

// EncodeSlice stores a slice as JSON. A nil slice stays blank so callers can
// tell "not generated yet" from an empty list.
func EncodeSlice[T any](items []T) string {
	if items == nil {
		return ""
	}
	b, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(b)
}

// ParseSlice restores a JSON slice. Blank text means the field was never set.
func ParseSlice[T any](raw string) []T {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var items []T
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

func firstText(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func usableLabel(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

func clipRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n]))
}
