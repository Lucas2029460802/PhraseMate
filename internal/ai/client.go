package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"phrasemate/internal/models"
)

// Client talks to an OpenAI-compatible Chat Completions API.
type Client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

// New creates an AI client.
func New(apiKey, baseURL, model string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Enabled reports whether an API key is configured.
func (c *Client) Enabled() bool {
	return c != nil && strings.TrimSpace(c.apiKey) != ""
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	Temperature    float64        `json:"temperature"`
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
		return nil, fmt.Errorf("未配置 API Key，请设置环境变量 PHRASEMATE_API_KEY 或 OPENAI_API_KEY")
	}

	system := `你是英语学习助手。用户会给出一个英语单词或短语。
请用 JSON 返回解释，字段如下（全部必填，字符串）：
{
  "term": "原词或短语（规范写法）",
  "phonetic": "音标，如 /həˈləʊ/，没有则空字符串",
  "part_of_speech": "词性，如 n. / v. / adj. / phrase，短语用 phrase",
  "meaning_en": "简洁的英文释义（1-2 句）",
  "meaning_zh": "准确的中文释义",
  "example_en": "一个自然的英文例句",
  "example_zh": "该例句的中文翻译"
}
只输出 JSON，不要 markdown 代码块或其它文字。`

	content, err := c.chat(ctx, system, term, true)
	if err != nil {
		return nil, err
	}

	content = stripCodeFence(content)
	var out models.AIExplanation
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("解析 AI 返回失败: %w\n原始内容: %s", err, content)
	}
	if strings.TrimSpace(out.Term) == "" {
		out.Term = term
	}
	return &out, nil
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
	reqBody := chatRequest{
		Model: c.model,
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

	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 AI 接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
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
