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
	if w.dictFirst && w.dict != nil {
		return true
	}
	return w.ai.Enabled()
}

func (w *Worker) explain(ctx context.Context, term string) (*models.AIExplanation, string, error) {
	if w.dictFirst && w.dict != nil && dict.IsSingleWord(term) {
		exp, err := w.dict.Lookup(ctx, term)
		if err == nil {
			return exp, "dict", nil
		}
		log.Printf("词典未命中 [%s]: %v，改用 AI", term, err)
	}

	if !w.ai.Enabled() {
		return nil, "", errNoEnricher
	}
	exp, err := w.ai.Explain(ctx, term)
	if err != nil {
		return nil, "", err
	}
	return exp, "ai", nil
}

var errNoEnricher = &enrichError{msg: "未配置 API Key，且词典未能提供释义"}

type enrichError struct {
	msg string
}

func (e *enrichError) Error() string {
	return e.msg
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
		if _, err := w.store.ApplyExplanation(item.ID, exp); err != nil {
			log.Printf("写入释义失败 [%s]: %v", item.Term, err)
			_ = w.store.MarkError(item.ID, err.Error())
		}
		if w.onDone != nil {
			w.onDone()
		}
	}
}
