package enricher

import (
	"context"
	"log"
	"time"

	"phrasemate/internal/ai"
	"phrasemate/internal/store"
)

// Worker fills pending vocabulary entries in the background.
type Worker struct {
	store  *store.Store
	ai     *ai.Client
	kick   chan struct{}
	onDone func()
}

// New creates an enricher worker.
func New(st *store.Store, client *ai.Client, onDone func()) *Worker {
	return &Worker{
		store:  st,
		ai:     client,
		kick:   make(chan struct{}, 1),
		onDone: onDone,
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

func (w *Worker) drain(ctx context.Context) {
	if !w.ai.Enabled() {
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
		exp, err := w.ai.Explain(ctx, item.Term)
		if err != nil {
			log.Printf("释义失败 [%s]: %v", item.Term, err)
			_ = w.store.MarkError(item.ID, err.Error())
			if w.onDone != nil {
				w.onDone()
			}
			continue
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
