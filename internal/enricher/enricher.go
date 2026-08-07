package enricher

import (
	"context"
	"log"
	"time"

	"phrasemate/internal/ai"
	"phrasemate/internal/dict"
	"phrasemate/internal/models"
	"phrasemate/internal/store"
)

// Worker fills pending vocabulary entries in the background.
type Worker struct {
	store     *store.Store
	ai        *ai.Client
	dict      *dict.Client
	dictFirst bool
	kick      chan struct{}
	onDone    func()
}

// New creates an enricher worker.
func New(st *store.Store, client *ai.Client, dictClient *dict.Client, dictFirst bool, onDone func()) *Worker {
	return &Worker{
		store:     st,
		ai:        client,
		dict:      dictClient,
		dictFirst: dictFirst,
		kick:      make(chan struct{}, 1),
		onDone:    onDone,
	}
}

// Kick asks the worker to process pending items soon.
func (w *Worker) Kick() {
	if w == nil {
		return
	}
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// Run blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	if w == nil {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.kick:
			w.drain(ctx)
		case <-ticker.C:
			w.drain(ctx)
		}
	}
}

func (w *Worker) canEnrich() bool {
	// Basic mode is always available even without API key.
	return true
}

func (w *Worker) explain(ctx context.Context, term string) (*models.AIExplanation, string, error) {
	singleWord := dict.IsSingleWord(term)
	if w.dictFirst && w.dict != nil && singleWord {
		exp, err := w.dict.Lookup(ctx, term)
		if err == nil {
			return exp, "dict", nil
		}
		log.Printf("词典未命中 [%s]: %v，改用 AI", term, err)
	}

	if !w.ai.Enabled() {
		return basicExplanation(term, singleWord), "basic", nil
	}
	exp, err := w.ai.Explain(ctx, term)
	if err != nil {
		return nil, "", err
	}
	return exp, "ai", nil
}

func basicExplanation(term string, singleWord bool) *models.AIExplanation {
	pos := "phrase"
	if singleWord {
		pos = "word"
	}
	return &models.AIExplanation{
		Term:         term,
		Phonetic:     "",
		PartOfSpeech: pos,
		MeaningEN:    "Basic mode: saved successfully. Add an API key for richer explanations and examples.",
		MeaningZH:    "基础版：已成功收录。请在应用「设置」中填写 API Key，以生成更完整的释义与例句。",
		ExampleEN:    "",
		ExampleZH:    "",
	}
}

func (w *Worker) drain(ctx context.Context) {
	if !w.canEnrich() {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		item, err := w.store.NextPending()
		if err != nil || item == nil {
			return
		}
		exp, source, err := w.explain(ctx, item.Term)
		if err != nil {
			log.Printf("释义失败 [%s]: %v", item.Term, err)
			_ = w.store.MarkError(item.ID, err.Error())
			if w.onDone != nil {
				w.onDone()
			}
			continue
		}
		if source == "dict" {
			log.Printf("词典释义 [%s]", item.Term)
		}
		if source == "basic" {
			log.Printf("基础版释义 [%s]", item.Term)
		}
		if _, err := w.store.ApplyExplanation(item.ID, exp); err != nil {
			log.Printf("写入释义失败 [%s]: %v", item.Term, err)
			_ = w.store.MarkError(item.ID, err.Error())
		}
		if w.onDone != nil {
			w.onDone()
		}
	}
}
