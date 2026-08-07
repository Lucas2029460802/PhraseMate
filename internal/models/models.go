package models

import "time"

// Word statuses.
const (
	StatusPending = "pending"
	StatusReady   = "ready"
	StatusError   = "error"
)

// Word is a vocabulary entry saved in the notebook.
type Word struct {
	ID           int64     `json:"id"`
	Term         string    `json:"term"`
	Phonetic     string    `json:"phonetic,omitempty"`
	MeaningEN    string    `json:"meaning_en"`
	MeaningZH    string    `json:"meaning_zh"`
	ExampleEN    string    `json:"example_en,omitempty"`
	ExampleZH    string    `json:"example_zh,omitempty"`
	PartOfSpeech string    `json:"part_of_speech,omitempty"`
	Status       string    `json:"status"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// CaptureRequest quickly saves a term without waiting for AI.
type CaptureRequest struct {
	Term string `json:"term"`
}

// LookupRequest is the payload for explaining a term (legacy sync path).
type LookupRequest struct {
	Term string `json:"term"`
}

// QuizRequest controls quiz generation.
type QuizRequest struct {
	Count int `json:"count"`
}

// QuizQuestion is a single self-test item.
type QuizQuestion struct {
	ID            int64    `json:"id"`
	Term          string   `json:"term"`
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectIndex  int      `json:"correct_index"`
	ExplainAnswer string   `json:"explain_answer"`
}

// QuizResponse wraps generated questions.
type QuizResponse struct {
	Questions []QuizQuestion `json:"questions"`
}

// StatusResponse reports runtime configuration.
type StatusResponse struct {
	OK        bool   `json:"ok"`
	HasKey    bool   `json:"has_key"`
	Model     string `json:"model"`
	BaseURL   string `json:"base_url"`
	WordCount int    `json:"word_count"`
	Pending   int    `json:"pending"`
}

// SettingsResponse is returned by GET /api/settings (never echoes full API key).
type SettingsResponse struct {
	HasKey       bool   `json:"has_key"`
	APIKeyMasked string `json:"api_key_masked"`
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
}

// SettingsUpdateRequest is the payload for PUT /api/settings.
type SettingsUpdateRequest struct {
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	ClearKey bool   `json:"clear_key"`
}

// AIExplanation is the structured reply from the LLM.
type AIExplanation struct {
	Term         string `json:"term"`
	Phonetic     string `json:"phonetic"`
	PartOfSpeech string `json:"part_of_speech"`
	MeaningEN    string `json:"meaning_en"`
	MeaningZH    string `json:"meaning_zh"`
	ExampleEN    string `json:"example_en"`
	ExampleZH    string `json:"example_zh"`
}
