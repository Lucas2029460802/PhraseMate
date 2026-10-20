package gitdata

import (
	"strings"
	"testing"
	"time"

	"phrasemate/internal/models"
)

func writingAt(id int64, prompt, body string, updated time.Time) models.Writing {
	return models.Writing{
		ID:        id,
		Prompt:    prompt,
		Body:      body,
		CreatedAt: updated.Add(-time.Hour),
		UpdatedAt: updated,
	}
}

func TestPrepareWritingsDedupesByID(t *testing.T) {
	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	got := prepareWritings([]models.Writing{
		writingAt(1, "Topic A", "old", at),
		writingAt(1, "Topic A", "new body", at.Add(time.Minute)),
		writingAt(0, "Orphan", "x", at),
	})
	if len(got) != 2 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].ID != 1 || got[0].Body != "new body" {
		t.Fatalf("id1 = %+v", got[0])
	}
	if got[1].ID <= 1 || got[1].Prompt != "Orphan" {
		t.Fatalf("orphan = %+v", got[1])
	}
}

func TestSelectWritingsSnapshotKeepsLocalWhenRemoteMissingFile(t *testing.T) {
	at := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	sqlite := []models.Writing{writingAt(1, "Essay", "draft", at)}
	remoteWordsOnly := []models.Writing{}

	got := selectWritingsSnapshot(sqlite, nil, false, false, remoteWordsOnly, true, false, relNone)
	if len(got) != 1 || got[0].Prompt != "Essay" {
		t.Fatalf("remote without writings.json should keep sqlite: %+v", got)
	}

	ahead := selectWritingsSnapshot(sqlite, nil, true, false, remoteWordsOnly, true, false, relRemoteAhead)
	if len(ahead) != 1 || ahead[0].Prompt != "Essay" {
		t.Fatalf("remote ahead without writings.json should keep sqlite: %+v", ahead)
	}

	explicitEmpty := selectWritingsSnapshot(sqlite, nil, false, false, remoteWordsOnly, true, true, relNone)
	if len(explicitEmpty) != 1 {
		// empty remote file + sqlite: still keep sqlite when remote list empty
		t.Fatalf("empty remote writings with sqlite: %+v", explicitEmpty)
	}

	remoteHas := []models.Writing{writingAt(2, "Remote", "ok", at)}
	taken := selectWritingsSnapshot(sqlite, nil, false, false, remoteHas, true, true, relNone)
	if len(taken) != 1 || taken[0].ID != 2 {
		t.Fatalf("remote with writings.json should win: %+v", taken)
	}
}

func TestMarshalWritingsRoundTrip(t *testing.T) {
	at := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	in := []models.Writing{{
		ID:            3,
		Prompt:        "Write about travel",
		Body:          "Once upon a time",
		ModelText:     "## Model\nGood essay.",
		ModelFilename: "model.md",
		PromptImages: []models.WritingImage{
			{Mime: "image/png", Data: "abc123"},
		},
		CreatedAt: at,
		UpdatedAt: at.Add(time.Minute),
	}}
	raw, err := marshalWritings(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"prompt": "Write about travel"`) {
		t.Fatalf("json = %s", raw)
	}
	if !strings.Contains(string(raw), `"image/png"`) {
		t.Fatalf("missing image: %s", raw)
	}
	out, err := unmarshalWritings(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !writingsEqual(in, out) {
		t.Fatalf("round trip %+v", out)
	}
}
