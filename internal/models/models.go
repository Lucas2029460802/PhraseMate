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
	ID           int64         `json:"id"`
	Term         string        `json:"term"`
	Phonetic     string        `json:"phonetic,omitempty"`
	AudioURL     string        `json:"audio_url,omitempty"`
	MeaningEN    string        `json:"meaning_en"`
	MeaningZH    string        `json:"meaning_zh"`
	ExampleEN    string        `json:"example_en,omitempty"`
	ExampleZH    string        `json:"example_zh,omitempty"`
	PartOfSpeech string        `json:"part_of_speech,omitempty"`
	RelatedForms []RelatedForm `json:"related_forms,omitempty"`
	Phrases      []Phrase      `json:"phrases,omitempty"`
	Status       string        `json:"status"`
	ErrorMsg     string        `json:"error_msg,omitempty"`
	QuizTested   int           `json:"quiz_tested,omitempty"`
	QuizWrong    int           `json:"quiz_wrong,omitempty"`
	QuizLastAt   string        `json:"quiz_last_at,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
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

// TranslateRequest is the payload for Chinese↔English translation.
type TranslateRequest struct {
	Text      string `json:"text"`
	Direction string `json:"direction"` // auto | en2zh | zh2en
}

// TranslateResponse is the AI translation result.
type TranslateResponse struct {
	SourceText  string `json:"source_text"`
	Translation string `json:"translation"`
	Direction   string `json:"direction"` // en2zh | zh2en
}

// Quiz question type constants.
const (
	QuizWordToDef      = "word_to_def"      // 看单词选英文释义
	QuizDefToWord      = "def_to_word"      // 看英文释义选单词
	QuizZhToWord       = "zh_to_word"       // 看中文释义选单词
	QuizWordToZh       = "word_to_zh"       // 看单词选中文释义
	QuizClosestMeaning = "closest_meaning" // 选最接近的英文释义
)

// QuizQuestion is a single self-test item.
type QuizQuestion struct {
	ID            int64    `json:"id"`
	Term          string   `json:"term"`
	Type          string   `json:"type,omitempty"`
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectIndex  int      `json:"correct_index"`
	ExplainAnswer string   `json:"explain_answer"`
}

// QuizResponse wraps generated questions.
type QuizResponse struct {
	Questions []QuizQuestion `json:"questions"`
}

// QuizAnswerResult is one answered item from the client.
type QuizAnswerResult struct {
	ID      int64 `json:"id"`
	Correct bool  `json:"correct"`
}

// QuizResultRequest reports quiz answers for spaced practice.
type QuizResultRequest struct {
	Results []QuizAnswerResult `json:"results"`
}

// StatusResponse reports runtime configuration.
type StatusResponse struct {
	OK         bool   `json:"ok"`
	HasKey     bool   `json:"has_key"`
	Model      string `json:"model"`
	BaseURL    string `json:"base_url"`
	WordCount  int    `json:"word_count"`
	Pending    int    `json:"pending"`
	DataBranch string `json:"data_branch,omitempty"`
	SyncError  string `json:"sync_error,omitempty"`
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
	Term         string        `json:"term"`
	Phonetic     string        `json:"phonetic"`
	AudioURL     string        `json:"audio_url,omitempty"`
	PartOfSpeech string        `json:"part_of_speech"`
	MeaningEN    string        `json:"meaning_en"`
	MeaningZH    string        `json:"meaning_zh"`
	ExampleEN    string        `json:"example_en"`
	ExampleZH    string        `json:"example_zh"`
	Forms        []RelatedForm `json:"forms,omitempty"`
	Phrases      []Phrase      `json:"phrases,omitempty"`
	// FamilyReady is true once related forms and phrases were requested.
	// Empty slices then mean "none", not "not yet generated".
	FamilyReady bool `json:"-"`
}

// RelatedForm is another part of speech built from the same root.
type RelatedForm struct {
	Word         string `json:"word"`
	Term         string `json:"term,omitempty"`
	POS          string `json:"pos"`
	PartOfSpeech string `json:"part_of_speech,omitempty"`
	MeaningZH    string `json:"meaning_zh"`
	Meaning      string `json:"meaning,omitempty"`
}

// Phrase is a short collocation that uses the headword.
type Phrase struct {
	Text      string `json:"phrase"`
	Alt       string `json:"text,omitempty"`
	MeaningZH string `json:"meaning_zh"`
	Meaning   string `json:"meaning,omitempty"`
}
