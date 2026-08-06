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

	"phrasemate/internal/models"
)

const defaultBaseURL = "https://api.dictionaryapi.dev/api/v2/entries/en"

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
			Timeout: 12 * time.Second,
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

	reqURL := c.baseURL + "/" + url.PathEscape(term)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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

	exp := buildExplanation(entries[0], term)
	if strings.TrimSpace(exp.MeaningEN) == "" {
		return nil, fmt.Errorf("dictionary: no definition")
	}

	zh, err := translateZH(ctx, c.http, exp.MeaningEN)
	if err != nil {
		return nil, fmt.Errorf("dictionary: translate meaning: %w", err)
	}
	exp.MeaningZH = zh

	if strings.TrimSpace(exp.ExampleEN) != "" {
		if exZh, err := translateZH(ctx, c.http, exp.ExampleEN); err == nil {
			exp.ExampleZH = exZh
		}
	}

	return exp, nil
}

func buildExplanation(entry apiEntry, fallbackTerm string) *models.AIExplanation {
	phonetic := strings.TrimSpace(entry.Phonetic)
	if phonetic == "" {
		for _, p := range entry.Phonetics {
			if t := strings.TrimSpace(p.Text); t != "" {
				phonetic = t
				break
			}
		}
	}

	var pos string
	var meaningEN string
	var exampleEN string

	for _, m := range entry.Meanings {
		if pos == "" && strings.TrimSpace(m.PartOfSpeech) != "" {
			pos = abbrevPOS(m.PartOfSpeech)
		}
		for _, d := range m.Definitions {
			def := strings.TrimSpace(d.Definition)
			if def == "" {
				continue
			}
			if meaningEN == "" {
				meaningEN = def
				exampleEN = strings.TrimSpace(d.Example)
				continue
			}
			if len(meaningEN) < 120 && len(meaningEN)+len(def)+2 < 200 {
				meaningEN += "; " + def
			}
			if exampleEN == "" {
				exampleEN = strings.TrimSpace(d.Example)
			}
			break
		}
		if meaningEN != "" {
			break
		}
	}

	term := strings.TrimSpace(entry.Word)
	if term == "" {
		term = fallbackTerm
	}
	if !strings.HasPrefix(phonetic, "/") && phonetic != "" && !strings.Contains(phonetic, "/") {
		phonetic = "/" + phonetic + "/"
	}

	return &models.AIExplanation{
		Term:         term,
		Phonetic:     phonetic,
		PartOfSpeech: pos,
		MeaningEN:    meaningEN,
		ExampleEN:    exampleEN,
	}
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
	q := url.Values{}
	q.Set("q", text)
	q.Set("langpair", "en|zh-CN")
	reqURL := "https://api.mymemory.translated.net/get?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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
	return out, nil
}
