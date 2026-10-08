package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"phrasemate/internal/format"
	"phrasemate/internal/models"
)

// Client talks to an OpenAI-compatible Chat Completions API.
type Client struct {
	mu      sync.RWMutex
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

// New creates an AI client.
func New(apiKey, baseURL, model string) *Client {
	return &Client{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		model:   strings.TrimSpace(model),
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Enabled reports whether an API key is configured.
func (c *Client) Enabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return strings.TrimSpace(c.apiKey) != ""
}

// UpdateCredentials hot-swaps API key / base URL / model.
func (c *Client) UpdateCredentials(apiKey, baseURL, model string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = strings.TrimSpace(apiKey)
	if u := strings.TrimRight(strings.TrimSpace(baseURL), "/"); u != "" {
		c.baseURL = u
	}
	if m := strings.TrimSpace(model); m != "" {
		c.model = m
	}
}

// APIKey returns the current API key.
func (c *Client) APIKey() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey
}

// BaseURL returns the current base URL.
func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL
}

// Model returns the current model name.
func (c *Client) Model() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.model
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Explain asks the model to explain a word or phrase in EN + ZH.
func (c *Client) Explain(ctx context.Context, term string) (*models.AIExplanation, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("未配置 API Key，请在应用设置中填写")
	}

	system := `你是英语学习助手。用户会给出一个英语单词或短语。
请用 JSON 返回解释，字段如下：
{
  "term": "原词或短语（规范写法）",
  "phonetic": "IPA 音标，格式必须为 /həˈləʊ/（斜杠包裹，不含词性或其它文字），没有则空字符串",
  "part_of_speech": "词性，如 n. / v. / adj. / phrase，短语用 phrase",
  "meaning_en": "简洁的英文释义（1-2 句）",
  "meaning_zh": "准确的中文释义（使用中文标点，多条释义用；分隔）",
  "example_en": "一个自然的英文例句",
  "example_zh": "该例句的中文翻译",
  "forms": [
    {"word": "同一词根的其他词性", "pos": "n. / v. / adj. / adv.", "meaning_zh": "简短中文"}
  ],
  "phrases": [
    {"phrase": "含该词的常用短语或固定搭配", "meaning_zh": "简短中文"}
  ]
}
规则：
- forms 给 2 到 4 个派生词，词性必须和原词不同。例如 beautiful 可给 beauty（n.）、beautify（v.）、beautifully（adv.）。不要比较级、过去式、现在分词、复数这类屈折变化，也不要重复原词。没有可靠派生词时返回空数组。
- phrases 给 2 到 4 个常见短语或搭配，每条附简短中文。不要整句例句，不要重复原词本身。
只输出 JSON，不要 markdown 代码块或其它文字。`

	content, err := c.chat(ctx, system, term, true)
	if err != nil {
		return nil, err
	}

	content = stripCodeFence(content)
	out, err := decodeExplanation(content)
	if err != nil {
		return nil, fmt.Errorf("解析 AI 返回失败: %w\n原始内容: %s", err, content)
	}
	if strings.TrimSpace(out.Term) == "" {
		out.Term = term
	}
	out.Phonetic = format.NormalizePhonetic(out.Phonetic)
	out.Forms = models.NormalizeForms(out.Term, out.Forms)
	out.Phrases = models.NormalizePhrases(out.Term, out.Phrases)
	out.FamilyReady = true
	if strings.TrimSpace(out.MeaningZH) == "" {
		return nil, fmt.Errorf("AI 未返回中文释义")
	}
	return out, nil
}

// Family asks only for derivations and collocations of a term we already explained.
func (c *Client) Family(ctx context.Context, term, pos, meaningZH string) ([]models.RelatedForm, []models.Phrase, error) {
	if !c.Enabled() {
		return nil, nil, fmt.Errorf("未配置 API Key，请在应用设置中填写")
	}
	system := `你是英语学习助手。根据给定单词或短语，只返回 JSON：
{
  "forms": [
    {"word": "同一词根的其他词性", "pos": "n. / v. / adj. / adv.", "meaning_zh": "简短中文"}
  ],
  "phrases": [
    {"phrase": "含该词的常用短语或固定搭配", "meaning_zh": "简短中文"}
  ]
}
规则：
- forms 给 2 到 4 个派生词，词性必须和原词不同。例如形容词 beautiful 对应名词 beauty、动词 beautify、副词 beautifully。不要比较级、过去式、现在分词、复数，也不要重复原词。没有可靠派生词时返回空数组。
- phrases 给 2 到 4 个常见短语或搭配，附简短中文。不要整句例句，不要重复原词本身。
只输出 JSON，不要 markdown。`
	user := fmt.Sprintf("单词或短语：%s\n词性：%s\n已知中文释义：%s", strings.TrimSpace(term), strings.TrimSpace(pos), strings.TrimSpace(meaningZH))
	content, err := c.chat(ctx, system, user, true)
	if err != nil {
		return nil, nil, err
	}
	content = stripCodeFence(content)
	forms, phrases, err := decodeFamily(content)
	if err != nil {
		return nil, nil, fmt.Errorf("解析词族失败: %w\n原始内容: %s", err, content)
	}
	return models.NormalizeForms(term, forms), models.NormalizePhrases(term, phrases), nil
}

func decodeExplanation(content string) (*models.AIExplanation, error) {
	var raw struct {
		Term         string          `json:"term"`
		Phonetic     string          `json:"phonetic"`
		PartOfSpeech string          `json:"part_of_speech"`
		MeaningEN    string          `json:"meaning_en"`
		MeaningZH    string          `json:"meaning_zh"`
		ExampleEN    string          `json:"example_en"`
		ExampleZH    string          `json:"example_zh"`
		Forms        json.RawMessage `json:"forms"`
		RelatedForms json.RawMessage `json:"related_forms"`
		Phrases      json.RawMessage `json:"phrases"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}
	forms := decodeForms(raw.Forms)
	if len(forms) == 0 {
		forms = decodeForms(raw.RelatedForms)
	}
	return &models.AIExplanation{
		Term:         raw.Term,
		Phonetic:     raw.Phonetic,
		PartOfSpeech: raw.PartOfSpeech,
		MeaningEN:    raw.MeaningEN,
		MeaningZH:    raw.MeaningZH,
		ExampleEN:    raw.ExampleEN,
		ExampleZH:    raw.ExampleZH,
		Forms:        forms,
		Phrases:      decodePhrases(raw.Phrases),
	}, nil
}

func decodeFamily(content string) ([]models.RelatedForm, []models.Phrase, error) {
	var raw struct {
		Forms        json.RawMessage `json:"forms"`
		RelatedForms json.RawMessage `json:"related_forms"`
		Phrases      json.RawMessage `json:"phrases"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, nil, err
	}
	forms := decodeForms(raw.Forms)
	if len(forms) == 0 {
		forms = decodeForms(raw.RelatedForms)
	}
	return forms, decodePhrases(raw.Phrases), nil
}

func decodeForms(raw json.RawMessage) []models.RelatedForm {
	if len(bytes.TrimSpace(raw)) == 0 || string(raw) == "null" {
		return nil
	}
	var forms []models.RelatedForm
	if err := json.Unmarshal(raw, &forms); err != nil {
		return nil
	}
	return forms
}

func decodePhrases(raw json.RawMessage) []models.Phrase {
	if len(bytes.TrimSpace(raw)) == 0 || string(raw) == "null" {
		return nil
	}
	var phrases []models.Phrase
	if err := json.Unmarshal(raw, &phrases); err != nil {
		return nil
	}
	return phrases
}

// GenerateQuiz builds multiple-choice questions from notebook entries.
func (c *Client) GenerateQuiz(ctx context.Context, words []models.Word, count int) ([]models.QuizQuestion, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("未配置 API Key")
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("生词本为空，请先添加生词")
	}
	if count <= 0 {
		count = 5
	}
	if count > len(words) {
		count = len(words)
	}
	if count > 10 {
		count = 10
	}

	type brief struct {
		ID        int64  `json:"id"`
		Term      string `json:"term"`
		MeaningZH string `json:"meaning_zh"`
		MeaningEN string `json:"meaning_en"`
	}
	briefs := make([]brief, 0, len(words))
	for _, w := range words {
		briefs = append(briefs, brief{ID: w.ID, Term: w.Term, MeaningZH: w.MeaningZH, MeaningEN: w.MeaningEN})
	}
	payload, _ := json.Marshal(briefs)

	system := fmt.Sprintf(`You are an English vocabulary quiz generator.
Create exactly %d multiple-choice questions from the user's notebook.
Return ONLY a JSON object:
{
  "questions": [
    {
      "id": <number, word id from input>,
      "term": "<English word or phrase>",
      "question": "<English stem only>",
      "options": ["A", "B", "C", "D"],
      "correct_index": <0-3>,
      "explain_answer": "<short English explanation>"
    }
  ]
}
Hard rules:
- Use ENGLISH only for question, options, and explain_answer. No Chinese characters.
- Prefer formats like: "Which definition best matches ___?" / "Choose the closest meaning of ___." / "___ is closest in meaning to:"
- Options must be English definitions or English paraphrases (exactly 4).
- Distractors must be plausible but incorrect.
- Output JSON only, no markdown.`, count)

	user := "Notebook entries (JSON):\n" + string(payload)
	content, err := c.chat(ctx, system, user, true)
	if err != nil {
		return nil, err
	}
	content = stripCodeFence(content)

	var wrapped struct {
		Questions []models.QuizQuestion `json:"questions"`
	}
	if err := json.Unmarshal([]byte(content), &wrapped); err == nil && len(wrapped.Questions) > 0 {
		return wrapped.Questions, nil
	}

	var questions []models.QuizQuestion
	if err := json.Unmarshal([]byte(content), &questions); err != nil {
		return nil, fmt.Errorf("解析测验题目失败: %w\n原始内容: %s", err, content)
	}
	return questions, nil
}

func (c *Client) chat(ctx context.Context, system, user string, jsonMode bool) (string, error) {
	c.mu.RLock()
	model := c.model
	baseURL := c.baseURL
	apiKey := c.apiKey
	c.mu.RUnlock()

	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.4,
	}
	if jsonMode {
		reqBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := baseURL + "/chat/completions"
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts && isTransientNetErr(err) && ctx.Err() == nil {
				sleepBackoff(ctx, attempt)
				continue
			}
			return "", fmt.Errorf("请求 AI 接口失败: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			if attempt < maxAttempts && isTransientNetErr(err) && ctx.Err() == nil {
				sleepBackoff(ctx, attempt)
				continue
			}
			return "", err
		}
		// Retry a few gateway / rate-limit responses that often clear quickly.
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout) &&
			attempt < maxAttempts && ctx.Err() == nil {
			lastErr = fmt.Errorf("AI 接口错误 (%d): %s", resp.StatusCode, string(body))
			sleepBackoff(ctx, attempt)
			continue
		}
		if resp.StatusCode >= 300 {
			return "", fmt.Errorf("AI 接口错误 (%d): %s", resp.StatusCode, string(body))
		}

		var parsed chatResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return "", fmt.Errorf("解析 AI 响应失败: %w", err)
		}
		if parsed.Error != nil {
			return "", fmt.Errorf("AI 错误: %s", parsed.Error.Message)
		}
		if len(parsed.Choices) == 0 {
			return "", fmt.Errorf("AI 未返回内容")
		}

		return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
	}
	return "", fmt.Errorf("请求 AI 接口失败: %w", lastErr)
}

func sleepBackoff(ctx context.Context, attempt int) {
	d := time.Duration(attempt) * 800 * time.Millisecond
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func isTransientNetErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, tip := range []string{
		"forcibly closed",
		"connection reset",
		"connection refused",
		"broken pipe",
		"i/o timeout",
		"tls handshake timeout",
		"unexpected eof",
		"wsarecv",
		"wsasend",
		"use of closed network connection",
		"temporary failure",
		"server misbehaving",
	} {
		if strings.Contains(msg, tip) {
			return true
		}
	}
	type temporary interface{ Temporary() bool }
	type timeout interface{ Timeout() bool }
	if t, ok := err.(temporary); ok && t.Temporary() {
		return true
	}
	if t, ok := err.(timeout); ok && t.Timeout() {
		return true
	}
	return false
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}
