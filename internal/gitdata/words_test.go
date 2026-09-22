package gitdata

import (
	"strings"
	"testing"
	"time"

	"phrasemate/internal/models"
)

func wordAt(id int64, term, zh, status string, at time.Time) models.Word {
	return models.Word{
		ID:        id,
		Term:      term,
		MeaningZH: zh,
		Status:    status,
		CreatedAt: at,
	}
}

func TestPrepareWordsDedupesAndAssignsIDs(t *testing.T) {
	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	got := prepareWords([]models.Word{
		wordAt(0, " Hello ", "", models.StatusPending, at),
		wordAt(4, "hello", "你好", models.StatusReady, at),
		wordAt(4, "World", "世界", models.StatusReady, at.Add(time.Minute)),
	})
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Term != "hello" || got[0].MeaningZH != "你好" || got[0].ID != 4 {
		t.Fatalf("hello = %+v", got[0])
	}
	if got[1].Term != "World" || got[1].ID == 4 {
		t.Fatalf("world = %+v", got[1])
	}
}

func TestSelectSnapshot(t *testing.T) {
	at := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	local := []models.Word{wordAt(1, "alpha", "甲", models.StatusReady, at)}
	sqlite := []models.Word{
		wordAt(1, "alpha", "甲", models.StatusReady, at),
		wordAt(2, "beta", "乙", models.StatusReady, at),
	}
	remote := []models.Word{wordAt(3, "gamma", "丙", models.StatusReady, at)}

	if got := selectSnapshot(sqlite, nil, false, nil, false, relNone); len(got) != 2 {
		t.Fatalf("no branch: %+v", got)
	}
	if got := selectSnapshot(sqlite, local, true, local, true, relEqual); !wordsEqual(got, sqlite) {
		t.Fatalf("equal but sqlite changed: %+v", got)
	}
	same := selectSnapshot(local, local, true, local, true, relEqual)
	if !wordsEqual(same, local) {
		t.Fatalf("equal snapshot: %+v", same)
	}
	restored := selectSnapshot(nil, local, true, local, true, relEqual)
	if !wordsEqual(restored, local) {
		t.Fatalf("empty sqlite should keep data branch: %+v", restored)
	}
	ahead := selectSnapshot(sqlite, local, true, local, true, relLocalAhead)
	if len(ahead) != 2 || ahead[1].Term != "beta" {
		t.Fatalf("local ahead: %+v", ahead)
	}
	behind := selectSnapshot(sqlite, local, true, remote, true, relRemoteAhead)
	if len(behind) != 1 || behind[0].Term != "gamma" {
		t.Fatalf("remote ahead: %+v", behind)
	}
	freshRemote := selectSnapshot(nil, nil, false, remote, true, relNone)
	if len(freshRemote) != 1 || freshRemote[0].Term != "gamma" {
		t.Fatalf("remote only: %+v", freshRemote)
	}
	kept := selectSnapshot(sqlite, nil, false, nil, true, relNone)
	if len(kept) != 2 {
		t.Fatalf("empty remote should keep sqlite, got %+v", kept)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	at := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	in := []models.Word{wordAt(1, "phrase", "短语", models.StatusReady, at)}
	raw, err := marshalWords(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"term": "phrase"`) {
		t.Fatalf("json = %s", raw)
	}
	out, err := unmarshalWords(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !wordsEqual(in, out) {
		t.Fatalf("round trip %+v", out)
	}
}
