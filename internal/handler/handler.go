package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"phrasemate/internal/ai"
	"phrasemate/internal/config"
	"phrasemate/internal/enricher"
	"phrasemate/internal/models"
	"phrasemate/internal/store"
)

// API exposes HTTP endpoints for PhraseMate.
type API struct {
	mu       sync.RWMutex
	cfg      config.Config
	store    *store.Store
	ai       *ai.Client
	enricher *enricher.Worker
}

// New creates an API handler.
func New(cfg config.Config, st *store.Store, client *ai.Client, en *enricher.Worker) *API {
	return &API{cfg: cfg, store: st, ai: client, enricher: en}
}

// Register mounts routes on mux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", a.handleStatus)
	mux.HandleFunc("GET /api/settings", a.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", a.handlePutSettings)
	mux.HandleFunc("POST /api/capture", a.handleCapture)
	mux.HandleFunc("POST /api/lookup", a.handleLookup)
	mux.HandleFunc("GET /api/words", a.handleListWords)
	mux.HandleFunc("DELETE /api/words/{id}", a.handleDeleteWord)
	mux.HandleFunc("POST /api/words/{id}/retry", a.handleRetry)
	mux.HandleFunc("POST /api/quiz", a.handleQuiz)
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	count, _ := a.store.Count()
	pending, _ := a.store.CountPending()
	a.mu.RLock()
	model := a.cfg.Model
	baseURL := a.cfg.BaseURL
	a.mu.RUnlock()
	if a.ai != nil {
		if m := a.ai.Model(); m != "" {
			model = m
		}
		if u := a.ai.BaseURL(); u != "" {
			baseURL = u
		}
	}
	writeJSON(w, http.StatusOK, models.StatusResponse{
		OK:        true,
		HasKey:    a.ai.Enabled(),
		Model:     model,
		BaseURL:   baseURL,
		WordCount: count,
		Pending:   pending,
	})
}

func (a *API) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	key := ""
	model := ""
	baseURL := ""
	if a.ai != nil {
		key = a.ai.APIKey()
		model = a.ai.Model()
		baseURL = a.ai.BaseURL()
	}
	a.mu.RLock()
	if model == "" {
		model = a.cfg.Model
	}
	if baseURL == "" {
		baseURL = a.cfg.BaseURL
	}
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, models.SettingsResponse{
		HasKey:       strings.TrimSpace(key) != "",
		APIKeyMasked: maskAPIKey(key),
		BaseURL:      baseURL,
		Model:        model,
	})
}

func (a *API) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req models.SettingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式无效")
		return
	}

	apiKey := ""
	if a.ai != nil {
		apiKey = a.ai.APIKey()
	}
	a.mu.RLock()
	baseURL := a.cfg.BaseURL
	model := a.cfg.Model
	a.mu.RUnlock()
	if a.ai != nil {
		if u := a.ai.BaseURL(); u != "" {
			baseURL = u
		}
		if m := a.ai.Model(); m != "" {
			model = m
		}
	}

	if req.ClearKey {
		apiKey = ""
	} else if strings.TrimSpace(req.APIKey) != "" {
		apiKey = strings.TrimSpace(req.APIKey)
	}

	if strings.TrimSpace(req.BaseURL) != "" {
		baseURL = config.NormalizeBaseURL(req.BaseURL)
	}
	if strings.TrimSpace(req.Model) != "" {
		model = strings.TrimSpace(req.Model)
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	if err := a.store.SetSetting(store.SettingAPIKey, apiKey); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存 API Key 失败")
		return
	}
	if err := a.store.SetSetting(store.SettingBaseURL, baseURL); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存 Base URL 失败")
		return
	}
	if err := a.store.SetSetting(store.SettingModel, model); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存模型失败")
		return
	}

	a.mu.Lock()
	a.cfg.APIKey = apiKey
	a.cfg.BaseURL = baseURL
	a.cfg.Model = model
	a.mu.Unlock()

	if a.ai != nil {
		a.ai.UpdateCredentials(apiKey, baseURL, model)
	}
	if a.enricher != nil && strings.TrimSpace(apiKey) != "" {
		a.enricher.Kick()
	}

	writeJSON(w, http.StatusOK, models.SettingsResponse{
		HasKey:       strings.TrimSpace(apiKey) != "",
		APIKeyMasked: maskAPIKey(apiKey),
		BaseURL:      baseURL,
		Model:        model,
	})
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "••••••••"
	}
	return key[:4] + "••••" + key[len(key)-4:]
}

func (a *API) handleCapture(w http.ResponseWriter, r *http.Request) {
	var req models.CaptureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式无效")
		return
	}
	term := strings.TrimSpace(req.Term)
	if term == "" {
		writeErr(w, http.StatusBadRequest, "请输入单词或短语")
		return
	}
	word, _, err := a.store.Capture(term)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if a.enricher != nil {
		a.enricher.Kick()
	}
	writeJSON(w, http.StatusOK, word)
}

func (a *API) handleLookup(w http.ResponseWriter, r *http.Request) {
	// Compat: capture + wait is no longer used by float; keep as capture kick.
	a.handleCapture(w, r)
}

func (a *API) handleListWords(w http.ResponseWriter, r *http.Request) {
	list, err := a.store.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []models.Word{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleDeleteWord(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "无效的生词 ID")
		return
	}
	if err := a.store.Delete(id); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) handleRetry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "无效的生词 ID")
		return
	}
	if err := a.store.Retry(id); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if a.enricher != nil {
		a.enricher.Kick()
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) handleQuiz(w http.ResponseWriter, r *http.Request) {
	var req models.QuizRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Count <= 0 {
		req.Count = 5
	}

	words, err := a.store.ListReady()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(words) < 2 {
		writeErr(w, http.StatusBadRequest, "至少需要 2 个已生成释义的生词才能出题")
		return
	}

	questions, err := a.ai.GenerateQuiz(r.Context(), words, req.Count)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, models.QuizResponse{Questions: questions})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
