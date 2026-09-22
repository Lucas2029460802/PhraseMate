package gitdata

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"phrasemate/internal/format"
	"phrasemate/internal/models"
)

const wordsFile = "words.json"

// Relation describes how the local data branch compares with the remote one.
type Relation int

const (
	relNone Relation = iota
	relEqual
	relLocalAhead
	relRemoteAhead
	relDiverged
)

type document struct {
	Version int       `json:"version"`
	Words   []wordRec `json:"words"`
}

type wordRec struct {
	ID           int64  `json:"id"`
	Term         string `json:"term"`
	Phonetic     string `json:"phonetic,omitempty"`
	MeaningEN    string `json:"meaning_en,omitempty"`
	MeaningZH    string `json:"meaning_zh,omitempty"`
	ExampleEN    string `json:"example_en,omitempty"`
	ExampleZH    string `json:"example_zh,omitempty"`
	PartOfSpeech string `json:"part_of_speech,omitempty"`
	Status       string `json:"status"`
	ErrorMsg     string `json:"error_msg,omitempty"`
	CreatedAt    string `json:"created_at"`
}

func prepareWords(in []models.Word) []models.Word {
	by := make(map[string]models.Word, len(in))
	for _, w := range in {
		w.Term = strings.TrimSpace(w.Term)
		key := strings.ToLower(w.Term)
		if key == "" {
			continue
		}
		w.Phonetic = format.NormalizePhonetic(w.Phonetic)
		w.Status = strings.TrimSpace(w.Status)
		if w.Status == "" {
			w.Status = models.StatusReady
		}
		if w.CreatedAt.IsZero() {
			w.CreatedAt = time.Unix(0, 0).UTC()
		} else {
			w.CreatedAt = w.CreatedAt.UTC().Truncate(time.Second)
		}
		prev, ok := by[key]
		if !ok {
			by[key] = w
			continue
		}
		if betterWord(w, prev) {
			if w.ID <= 0 {
				w.ID = prev.ID
			}
			by[key] = w
			continue
		}
		if prev.ID <= 0 && w.ID > 0 {
			prev.ID = w.ID
			by[key] = prev
		}
	}

	out := make([]models.Word, 0, len(by))
	used := make(map[int64]struct{}, len(by))
	var maxID int64
	for _, w := range by {
		if w.ID > maxID {
			maxID = w.ID
		}
		out = append(out, w)
	}
	sortWords(out)
	for i := range out {
		if out[i].ID > 0 {
			if _, dup := used[out[i].ID]; !dup {
				used[out[i].ID] = struct{}{}
				continue
			}
		}
		maxID++
		out[i].ID = maxID
		used[out[i].ID] = struct{}{}
	}
	sortWords(out)
	return out
}

func sortWords(out []models.Word) {
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && wordLess(out[j], out[j-1]) {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
}

func wordLess(a, b models.Word) bool {
	if a.ID != b.ID {
		if a.ID <= 0 {
			return false
		}
		if b.ID <= 0 {
			return true
		}
		return a.ID < b.ID
	}
	return strings.ToLower(a.Term) < strings.ToLower(b.Term)
}

func betterWord(a, b models.Word) bool {
	if a.CreatedAt.After(b.CreatedAt) {
		return true
	}
	if b.CreatedAt.After(a.CreatedAt) {
		return false
	}
	if ra, rb := statusRank(a.Status), statusRank(b.Status); ra != rb {
		return ra > rb
	}
	aLen := len(strings.TrimSpace(a.MeaningZH)) + len(strings.TrimSpace(a.MeaningEN))
	bLen := len(strings.TrimSpace(b.MeaningZH)) + len(strings.TrimSpace(b.MeaningEN))
	return aLen > bLen
}

func statusRank(status string) int {
	switch status {
	case models.StatusReady:
		return 3
	case models.StatusError:
		return 2
	default:
		return 1
	}
}

func mergeWords(sets ...[]models.Word) []models.Word {
	var all []models.Word
	for _, set := range sets {
		all = append(all, set...)
	}
	return prepareWords(all)
}

// selectSnapshot picks the vocabulary that should be stored.
// A complete snapshot on the data branch is the record of deletions.
// SQLite is only preferred when it diverged from the last synced commit,
// or when the remote branch does not exist yet.
func selectSnapshot(sqlite []models.Word, local []models.Word, hasLocal bool, remote []models.Word, hasRemote bool, rel Relation) []models.Word {
	switch {
	case !hasLocal && !hasRemote:
		return prepareWords(sqlite)
	case !hasLocal && hasRemote:
		if len(prepareWords(remote)) == 0 && len(prepareWords(sqlite)) > 0 {
			return prepareWords(sqlite)
		}
		return prepareWords(remote)
	case hasLocal && hasRemote && rel == relEqual:
		if len(prepareWords(sqlite)) == 0 && len(prepareWords(local)) > 0 {
			return prepareWords(local)
		}
		if wordsEqual(sqlite, local) {
			return prepareWords(local)
		}
		return prepareWords(sqlite)
	case hasLocal && rel == relLocalAhead:
		return mergeWords(local, sqlite)
	case hasRemote && rel == relRemoteAhead:
		return prepareWords(remote)
	case hasLocal && hasRemote && rel == relDiverged:
		return mergeWords(remote, local, sqlite)
	case hasLocal:
		return mergeWords(local, sqlite)
	default:
		return prepareWords(remote)
	}
}

func parentFor(localHash string, hasLocal bool, remoteHash string, hasRemote bool, rel Relation) string {
	switch {
	case rel == relLocalAhead && hasLocal:
		return localHash
	case hasRemote && (rel == relRemoteAhead || rel == relDiverged || rel == relEqual || !hasLocal):
		return remoteHash
	case hasLocal:
		return localHash
	default:
		return ""
	}
}

func marshalWords(words []models.Word) ([]byte, error) {
	recs := make([]wordRec, 0, len(words))
	for _, w := range prepareWords(words) {
		recs = append(recs, wordRec{
			ID:           w.ID,
			Term:         w.Term,
			Phonetic:     w.Phonetic,
			MeaningEN:    w.MeaningEN,
			MeaningZH:    w.MeaningZH,
			ExampleEN:    w.ExampleEN,
			ExampleZH:    w.ExampleZH,
			PartOfSpeech: w.PartOfSpeech,
			Status:       w.Status,
			ErrorMsg:     w.ErrorMsg,
			CreatedAt:    w.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	buf, err := json.MarshalIndent(document{Version: 1, Words: recs}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(buf, '\n'), nil
}

func unmarshalWords(raw []byte) ([]models.Word, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return []models.Word{}, nil
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		var list []wordRec
		if err2 := json.Unmarshal(raw, &list); err2 != nil {
			return nil, err
		}
		doc.Words = list
	}
	out := make([]models.Word, 0, len(doc.Words))
	for _, rec := range doc.Words {
		created, _ := time.Parse(time.RFC3339, rec.CreatedAt)
		out = append(out, models.Word{
			ID:           rec.ID,
			Term:         rec.Term,
			Phonetic:     rec.Phonetic,
			MeaningEN:    rec.MeaningEN,
			MeaningZH:    rec.MeaningZH,
			ExampleEN:    rec.ExampleEN,
			ExampleZH:    rec.ExampleZH,
			PartOfSpeech: rec.PartOfSpeech,
			Status:       rec.Status,
			ErrorMsg:     rec.ErrorMsg,
			CreatedAt:    created,
		})
	}
	return prepareWords(out), nil
}

func wordsEqual(a, b []models.Word) bool {
	ab, err1 := marshalWords(a)
	bb, err2 := marshalWords(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}
