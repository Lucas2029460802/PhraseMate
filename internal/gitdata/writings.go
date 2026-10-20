package gitdata

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"phrasemate/internal/models"
)

const writingsFile = "writings.json"

type writingsDocument struct {
	Version  int          `json:"version"`
	Writings []writingRec `json:"writings"`
}

type writingRec struct {
	ID            int64                 `json:"id"`
	Prompt        string                `json:"prompt"`
	PromptImages  []models.WritingImage `json:"prompt_images,omitempty"`
	Body          string                `json:"body"`
	ModelText     string                `json:"model_text"`
	ModelFilename string                `json:"model_filename,omitempty"`
	CreatedAt     string                `json:"created_at"`
	UpdatedAt     string                `json:"updated_at"`
}

func prepareWritings(in []models.Writing) []models.Writing {
	by := make(map[int64]models.Writing, len(in))
	var orphans []models.Writing
	var maxID int64
	for _, w := range in {
		w.Prompt = strings.TrimSpace(w.Prompt)
		w.ModelFilename = strings.TrimSpace(w.ModelFilename)
		if w.CreatedAt.IsZero() {
			w.CreatedAt = time.Unix(0, 0).UTC()
		} else {
			w.CreatedAt = w.CreatedAt.UTC().Truncate(time.Second)
		}
		if w.UpdatedAt.IsZero() {
			w.UpdatedAt = w.CreatedAt
		} else {
			w.UpdatedAt = w.UpdatedAt.UTC().Truncate(time.Second)
		}
		if w.PromptImages == nil {
			w.PromptImages = []models.WritingImage{}
		}
		w.PromptImageCount = len(w.PromptImages)
		if w.ID <= 0 {
			orphans = append(orphans, w)
			continue
		}
		if w.ID > maxID {
			maxID = w.ID
		}
		prev, ok := by[w.ID]
		if !ok || betterWriting(w, prev) {
			by[w.ID] = w
		}
	}
	for _, w := range orphans {
		maxID++
		w.ID = maxID
		by[w.ID] = w
	}
	out := make([]models.Writing, 0, len(by))
	for _, w := range by {
		out = append(out, w)
	}
	sortWritings(out)
	return out
}

func sortWritings(out []models.Writing) {
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && writingLess(out[j], out[j-1]) {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
}

func writingLess(a, b models.Writing) bool {
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.Before(b.UpdatedAt)
	}
	return a.Prompt < b.Prompt
}

func betterWriting(a, b models.Writing) bool {
	if a.UpdatedAt.After(b.UpdatedAt) {
		return true
	}
	if b.UpdatedAt.After(a.UpdatedAt) {
		return false
	}
	aLen := len(a.Prompt) + len(a.Body) + len(a.ModelText) + len(a.PromptImages)
	bLen := len(b.Prompt) + len(b.Body) + len(b.ModelText) + len(b.PromptImages)
	return aLen > bLen
}

func mergeWritings(sets ...[]models.Writing) []models.Writing {
	var all []models.Writing
	for _, set := range sets {
		all = append(all, set...)
	}
	return prepareWritings(all)
}

// selectWritingsSnapshot mirrors vocabulary selection for the writing notebook.
// hasWritingsFile distinguishes an old data commit (no writings.json yet) from
// an explicit empty notebook, so local essays are not wiped during migration.
func selectWritingsSnapshot(
	sqlite []models.Writing,
	local []models.Writing, hasLocal, localHasFile bool,
	remote []models.Writing, hasRemote, remoteHasFile bool,
	rel Relation,
) []models.Writing {
	switch {
	case !hasLocal && !hasRemote:
		return prepareWritings(sqlite)
	case !hasLocal && hasRemote:
		if (!remoteHasFile || len(prepareWritings(remote)) == 0) && len(prepareWritings(sqlite)) > 0 {
			return prepareWritings(sqlite)
		}
		return prepareWritings(remote)
	case hasLocal && hasRemote && rel == relEqual:
		if len(prepareWritings(sqlite)) == 0 && len(prepareWritings(local)) > 0 {
			return prepareWritings(local)
		}
		if !localHasFile && len(prepareWritings(sqlite)) > 0 {
			return prepareWritings(sqlite)
		}
		if writingsEqual(sqlite, local) {
			return prepareWritings(local)
		}
		return prepareWritings(sqlite)
	case hasLocal && rel == relLocalAhead:
		return mergeWritings(local, sqlite)
	case hasRemote && rel == relRemoteAhead:
		if !remoteHasFile && len(prepareWritings(sqlite)) > 0 {
			return mergeWritings(sqlite, local)
		}
		return prepareWritings(remote)
	case hasLocal && hasRemote && rel == relDiverged:
		if !remoteHasFile && !localHasFile {
			return prepareWritings(sqlite)
		}
		return mergeWritings(remote, local, sqlite)
	case hasLocal:
		return mergeWritings(local, sqlite)
	default:
		if !remoteHasFile && len(prepareWritings(sqlite)) > 0 {
			return prepareWritings(sqlite)
		}
		return prepareWritings(remote)
	}
}

func marshalWritings(writings []models.Writing) ([]byte, error) {
	recs := make([]writingRec, 0, len(writings))
	for _, w := range prepareWritings(writings) {
		imgs := w.PromptImages
		if len(imgs) == 0 {
			imgs = nil
		}
		recs = append(recs, writingRec{
			ID:            w.ID,
			Prompt:        w.Prompt,
			PromptImages:  imgs,
			Body:          w.Body,
			ModelText:     w.ModelText,
			ModelFilename: w.ModelFilename,
			CreatedAt:     w.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:     w.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	buf, err := json.MarshalIndent(writingsDocument{Version: 1, Writings: recs}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(buf, '\n'), nil
}

func unmarshalWritings(raw []byte) ([]models.Writing, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return []models.Writing{}, nil
	}
	var doc writingsDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		var list []writingRec
		if err2 := json.Unmarshal(raw, &list); err2 != nil {
			return nil, err
		}
		doc.Writings = list
	}
	out := make([]models.Writing, 0, len(doc.Writings))
	for _, rec := range doc.Writings {
		created, _ := time.Parse(time.RFC3339, rec.CreatedAt)
		updated, _ := time.Parse(time.RFC3339, rec.UpdatedAt)
		imgs := rec.PromptImages
		if imgs == nil {
			imgs = []models.WritingImage{}
		}
		out = append(out, models.Writing{
			ID:               rec.ID,
			Prompt:           rec.Prompt,
			PromptImages:     imgs,
			PromptImageCount: len(imgs),
			Body:             rec.Body,
			ModelText:        rec.ModelText,
			ModelFilename:    rec.ModelFilename,
			CreatedAt:        created,
			UpdatedAt:        updated,
		})
	}
	return prepareWritings(out), nil
}

func writingsEqual(a, b []models.Writing) bool {
	ab, err1 := marshalWritings(a)
	bb, err2 := marshalWritings(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}
