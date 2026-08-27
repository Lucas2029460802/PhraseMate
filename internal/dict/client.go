package dict

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"phrasemate/internal/format"
	"phrasemate/internal/models"
)

const defaultBaseURL = "https://api.dictionaryapi.dev/api/v2/entries/en"
const (
	dictTimeout      = 3 * time.Second
	translateTimeout = 2 * time.Second
)

var singleWordRE = regexp.MustCompile(`^[a-zA-Z]+(?:[-'][a-zA-Z]+)*$`)

// Client queries free dictionary APIs for English vocabulary.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a dictionary client.
func New(baseURL string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: dictTimeout,
		},
	}
}

// IsSingleWord reports whether term looks like a single English word (not a phrase).
func IsSingleWord(term string) bool {
	term = strings.TrimSpace(term)
	if term == "" {
		return false
	}
	return singleWordRE.MatchString(term)
}

type apiEntry struct {
	Word      string `json:"word"`
	Phonetic  string `json:"phonetic"`
	Phonetics []struct {
		Text string `json:"text"`
	} `json:"phonetics"`
	Meanings []struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string `json:"definition"`
			Example    string `json:"example"`
		} `json:"definitions"`
	} `json:"meanings"`
}

// Lookup fetches an explanation from the free dictionary API.
// Returns an error when the word is not found or the response is unusable.
func (c *Client) Lookup(ctx context.Context, term string) (*models.AIExplanation, error) {
	if c == nil {
		return nil, fmt.Errorf("dictionary client is nil")
	}
	term = strings.TrimSpace(term)
	if !IsSingleWord(term) {
		return nil, fmt.Errorf("not a single word")
	}

	dictCtx, cancel := context.WithTimeout(ctx, dictTimeout)
	defer cancel()

	reqURL := c.baseURL + "/" + url.PathEscape(term)
	req, err := http.NewRequestWithContext(dictCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("dictionary: not found")
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dictionary: HTTP %d", resp.StatusCode)
	}

	var entries []apiEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("dictionary: parse failed: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("dictionary: empty response")
	}

	exp, defs := buildExplanation(entries[0], term)
	if strings.TrimSpace(exp.MeaningEN) == "" {
		return nil, fmt.Errorf("dictionary: no definition")
	}

	if zh, err := translateDefinitionsZH(ctx, c.http, defs); err == nil {
		exp.MeaningZH = zh
	}

	if strings.TrimSpace(exp.ExampleEN) != "" {
		if exZh, err := translateZH(ctx, c.http, exp.ExampleEN); err == nil {
			exp.ExampleZH = normalizeZH(exZh)
		}
	}

	return exp, nil
}

func buildExplanation(entry apiEntry, fallbackTerm string) (*models.AIExplanation, []string) {
	phonetic := pickPhonetic(entry)

	var pos string
	var defs []string
	var exampleEN string

	for _, m := range entry.Meanings {
		if pos == "" && strings.TrimSpace(m.PartOfSpeech) != "" {
			pos = abbrevPOS(m.PartOfSpeech)
		}
		for _, d := range m.Definitions {
			def := trimTrailingPunct(strings.TrimSpace(d.Definition))
			if def == "" {
				continue
			}
			if len(defs) == 0 {
				defs = append(defs, def)
				exampleEN = strings.TrimSpace(d.Example)
				continue
			}
			joined := strings.Join(defs, "; ")
			if len(joined) < 120 && len(joined)+len(def)+2 < 200 {
				defs = append(defs, def)
			}
			if exampleEN == "" {
				exampleEN = strings.TrimSpace(d.Example)
			}
			break
		}
		if len(defs) > 0 {
			break
		}
	}

	term := strings.TrimSpace(entry.Word)
	if term == "" {
		term = fallbackTerm
	}

	return &models.AIExplanation{
		Term:         term,
		Phonetic:     phonetic,
		PartOfSpeech: pos,
		MeaningEN:    strings.Join(defs, "; "),
		ExampleEN:    exampleEN,
	}, defs
}

func pickPhonetic(entry apiEntry) string {
	candidates := make([]string, 0, len(entry.Phonetics)+1)
	if p := strings.TrimSpace(entry.Phonetic); p != "" {
		candidates = append(candidates, p)
	}
	for _, ph := range entry.Phonetics {
		if t := strings.TrimSpace(ph.Text); t != "" {
			candidates = append(candidates, t)
		}
	}

	best := ""
	bestScore := -1
	for _, c := range candidates {
		norm := format.NormalizePhonetic(c)
		if norm == "" {
			continue
		}
		if s := scorePhoneticCandidate(c); s > bestScore {
			bestScore = s
			best = norm
		}
	}
	return best
}

func scorePhoneticCandidate(p string) int {
	score := 0
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "[") {
		score += 2
	}
	ipaMarkers := "əɪʊæɔθðŋʃʒˈˌː"
	for _, r := range p {
		if strings.ContainsRune(ipaMarkers, r) {
			score += 3
		}
	}
	return score
}

func trimTrailingPunct(s string) string {
	s = strings.TrimSpace(s)
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r == '.' || r == ';' || r == ',' {
			s = strings.TrimSpace(s[:len(s)-size])
			continue
		}
		break
	}
	return s
}

func translateDefinitionsZH(ctx context.Context, httpClient *http.Client, defs []string) (string, error) {
	if len(defs) == 0 {
		return "", fmt.Errorf("no definitions")
	}
	if len(defs) == 1 {
		zh, err := translateZH(ctx, httpClient, defs[0])
		if err != nil {
			return "", err
		}
		return normalizeZH(zh), nil
	}

	parts := make([]string, 0, len(defs))
	for _, def := range defs {
		zh, err := translateZH(ctx, httpClient, def)
		if err != nil {
			return "", err
		}
		parts = append(parts, normalizeZH(zh))
	}
	return strings.Join(parts, "；"), nil
}

func normalizeZH(s string) string {
	s = strings.TrimSpace(s)
	s = trimTrailingPunct(s)
	s = strings.ReplaceAll(s, "；。", "；")
	s = strings.ReplaceAll(s, "。；", "；")
	for strings.Contains(s, "。。") {
		s = strings.ReplaceAll(s, "。。", "。")
	}
	return s
}

func abbrevPOS(pos string) string {
	switch strings.ToLower(strings.TrimSpace(pos)) {
	case "noun":
		return "n."
	case "verb":
		return "v."
	case "adjective":
		return "adj."
	case "adverb":
		return "adv."
	case "preposition":
		return "prep."
	case "conjunction":
		return "conj."
	case "pronoun":
		return "pron."
	case "interjection", "exclamation":
		return "int."
	case "determiner":
		return "det."
	default:
		r := []rune(pos)
		if len(r) > 0 {
			r[0] = unicode.ToLower(r[0])
		}
		return string(r) + "."
	}
}

type myMemoryResponse struct {
	ResponseData struct {
		TranslatedText string `json:"translatedText"`
	} `json:"responseData"`
	ResponseStatus int `json:"responseStatus"`
}

func translateZH(ctx context.Context, httpClient *http.Client, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty text")
	}
	translateCtx, cancel := context.WithTimeout(ctx, translateTimeout)
	defer cancel()

	q := url.Values{}
	q.Set("q", text)
	q.Set("langpair", "en|zh-CN")
	reqURL := "https://api.mymemory.translated.net/get?" + q.Encode()

	req, err := http.NewRequestWithContext(translateCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("translate HTTP %d", resp.StatusCode)
	}

	var parsed myMemoryResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.ResponseStatus != 200 {
		return "", fmt.Errorf("translate status %d", parsed.ResponseStatus)
	}
	out := strings.TrimSpace(parsed.ResponseData.TranslatedText)
	if out == "" {
		return "", fmt.Errorf("empty translation")
	}
	if translationLooksFailed(text, out) {
		return "", fmt.Errorf("translation returned source text")
	}
	return out, nil
}

func translationLooksFailed(original, translated string) bool {
	if strings.EqualFold(strings.TrimSpace(original), strings.TrimSpace(translated)) {
		return true
	}
	runes := []rune(translated)
	if len(runes) == 0 {
		return true
	}
	asciiLetters := 0
	for _, r := range runes {
		if r < 128 && unicode.IsLetter(r) {
			asciiLetters++
		}
	}
	return asciiLetters*100/len(runes) > 85
}
