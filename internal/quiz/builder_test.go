package quiz

import (
	"strings"
	"testing"

	"phrasemate/internal/models"
)

func sampleWords() []models.Word {
	return []models.Word{
		{ID: 1, Term: "serendipity", MeaningEN: "a fortunate discovery by chance", MeaningZH: "意外发现好运", QuizTested: 0},
		{ID: 2, Term: "recession", MeaningEN: "a temporary economic decline", MeaningZH: "经济衰退", QuizTested: 3, QuizWrong: 2},
		{ID: 3, Term: "mandate", MeaningEN: "an official order or commission", MeaningZH: "授权；命令", QuizTested: 1, QuizWrong: 0},
		{ID: 4, Term: "animosity", MeaningEN: "strong hostility", MeaningZH: "敌意", QuizTested: 0},
	}
}

func TestBuildFillsTargetInStem(t *testing.T) {
	qs, err := Build(sampleWords(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) == 0 {
		t.Fatal("expected questions")
	}
	for _, q := range qs {
		if strings.Contains(q.Question, "___") {
			t.Fatalf("blank placeholder left in question: %q", q.Question)
		}
		if strings.TrimSpace(q.Term) == "" {
			t.Fatalf("missing term: %+v", q)
		}
		if len(q.Options) < 2 {
			t.Fatalf("need options: %+v", q)
		}
		if q.CorrectIndex < 0 || q.CorrectIndex >= len(q.Options) {
			t.Fatalf("bad correct_index: %+v", q)
		}
		switch q.Type {
		case models.QuizWordToDef, models.QuizClosestMeaning, models.QuizWordToZh:
			if !strings.Contains(strings.ToLower(q.Question), strings.ToLower(q.Term)) {
				t.Fatalf("stem missing term %q: %q", q.Term, q.Question)
			}
		case models.QuizDefToWord, models.QuizZhToWord:
			found := false
			for _, o := range q.Options {
				if strings.EqualFold(o, q.Term) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("options missing term for def-to-word: %+v", q)
			}
		}
	}
}

func TestBuildPrefersUntested(t *testing.T) {
	words := sampleWords()
	qs, err := Build(words, 2)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[int64]bool{}
	for _, q := range qs {
		ids[q.ID] = true
	}
	// With 2 untested (1,4), both slots should come from untested when count=2.
	for id := range ids {
		if id != 1 && id != 4 {
			// flaky if weighted somehow picks tested when untested exist for count=2
			// pickWeighted takes all untested first
			t.Fatalf("expected only untested words, got id=%d in %v", id, ids)
		}
	}
}
