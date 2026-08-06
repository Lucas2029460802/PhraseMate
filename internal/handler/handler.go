package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"phrasemate/internal/ai"
	"phrasemate/internal/config"
	"phrasemate/internal/enricher"
	"phrasemate/internal/models"
	"phrasemate/internal/store"
)

// API exposes HTTP endpoints for PhraseMate.
type API struct {
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
	writeJSON(w, http.StatusOK, models.StatusResponse{
		OK:        true,
		HasKey:    a.ai.Enabled(),
		Model:     a.cfg.Model,
		BaseURL:   a.cfg.BaseURL,
		WordCount: count,
		Pending:   pending,
	})
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
