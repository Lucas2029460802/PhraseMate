package gitdata

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"phrasemate/internal/models"
	"phrasemate/internal/store"
)

func TestPublishWordsToDataBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	bare := t.TempDir()
	gitRun(t, root, "init", "-b", "main")
	gitRun(t, bare, "init", "--bare", "-b", "main")
	gitRun(t, root, "remote", "add", "origin", bare)

	gs, err := openAt(root, "origin", "data")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "phrasemate.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetSetting(store.SettingAPIKey, "sk-secret-test"); err != nil {
		t.Fatal(err)
	}
	word, _, err := st.Capture("luminous")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ApplyExplanation(word.ID, &models.AIExplanation{
		Term:        "luminous",
		MeaningZH:   "发光的",
		MeaningEN:   "full of light",
		Phonetic:    "ˈluːmɪnəs",
		FamilyReady: true,
		Forms: []models.RelatedForm{
			{Word: "luminosity", POS: "n.", MeaningZH: "发光"},
		},
		Phrases: []models.Phrase{
			{Text: "luminous color", MeaningZH: "明亮的颜色"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	essay, err := st.CreateWriting("Travel essay", "My draft body", "## Model\nSample", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := gs.Bootstrap(st); err != nil {
		t.Fatal(err)
	}
	if err := gs.Flush(); err != nil {
		t.Fatal(err)
	}

	body := gitRun(t, bare, "show", "data:words.json")
	if !strings.Contains(body, "luminous") || !strings.Contains(body, "发光的") || !strings.Contains(body, "luminosity") || !strings.Contains(body, "luminous color") {
		t.Fatalf("words.json = %s", body)
	}
	if strings.Contains(body, "sk-secret-test") {
		t.Fatal("API key leaked into data branch")
	}
	writingsBody := gitRun(t, bare, "show", "data:writings.json")
	if !strings.Contains(writingsBody, "Travel essay") || !strings.Contains(writingsBody, "My draft body") {
		t.Fatalf("writings.json = %s", writingsBody)
	}
	if strings.Contains(writingsBody, "sk-secret-test") {
		t.Fatal("API key leaked into writings.json")
	}
	status := gitRun(t, root, "status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Fatalf("main worktree changed: %s", status)
	}

	st2, err := store.Open(filepath.Join(t.TempDir(), "other.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	if err := gs.Bootstrap(st2); err != nil {
		t.Fatal(err)
	}
	if err := gs.Flush(); err != nil {
		t.Fatal(err)
	}
	list, err := st2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Term != "luminous" || list[0].MeaningZH != "发光的" {
		t.Fatalf("restored = %+v", list)
	}
	if len(list[0].RelatedForms) != 1 || list[0].RelatedForms[0].Word != "luminosity" {
		t.Fatalf("forms = %+v", list[0].RelatedForms)
	}
	if len(list[0].Phrases) != 1 || list[0].Phrases[0].Text != "luminous color" {
		t.Fatalf("phrases = %+v", list[0].Phrases)
	}
	restoredWritings, err := st2.ListWritingsFull()
	if err != nil {
		t.Fatal(err)
	}
	if len(restoredWritings) != 1 || restoredWritings[0].Prompt != essay.Prompt || restoredWritings[0].Body != essay.Body {
		t.Fatalf("restored writings = %+v", restoredWritings)
	}
}

func TestBootstrapKeepsLocalWritingsWhenDataBranchOnlyHasWords(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	bare := t.TempDir()
	gitRun(t, root, "init", "-b", "main")
	gitRun(t, bare, "init", "--bare", "-b", "main")
	gitRun(t, root, "remote", "add", "origin", bare)

	gs, err := openAt(root, "origin", "data")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	word, _, err := st.Capture("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ApplyExplanation(word.ID, &models.AIExplanation{
		Term: "alpha", MeaningZH: "甲", MeaningEN: "first", FamilyReady: true,
	}); err != nil {
		t.Fatal(err)
	}
	// Publish words-only by committing through Save after stripping writings is not needed:
	// first Bootstrap writes both files. Simulate legacy remote by committing words only.
	if err := gs.Bootstrap(st); err != nil {
		t.Fatal(err)
	}
	if err := gs.Flush(); err != nil {
		t.Fatal(err)
	}

	// Create a words-only tree on data (legacy shape before writings.json existed).
	wordsJSON := gitRun(t, root, "show", "data:words.json")
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(wordsJSON)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=PhraseMate Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=PhraseMate Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_TERMINAL_PROMPT=0",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	wordsBlob := strings.TrimSpace(string(out))
	cmd = exec.Command("git", "mktree")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader("100644 blob " + wordsBlob + "\twords.json\n")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.TrimSpace(string(out))
	commit := strings.TrimSpace(gitRun(t, root, "commit-tree", tree, "-m", "legacy words only"))
	gitRun(t, root, "update-ref", "refs/heads/data", commit)
	gitRun(t, root, "push", "--force", "origin", "refs/heads/data:refs/heads/data")

	stLocal, err := store.Open(filepath.Join(t.TempDir(), "with-essay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer stLocal.Close()
	if _, err := stLocal.CreateWriting("Keep me", "local essay", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := gs.Bootstrap(stLocal); err != nil {
		t.Fatal(err)
	}
	list, err := stLocal.ListWritingsFull()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Prompt != "Keep me" {
		t.Fatalf("local writings wiped: %+v", list)
	}
	if err := gs.Flush(); err != nil {
		t.Fatal(err)
	}
	published := gitRun(t, bare, "show", "data:writings.json")
	if !strings.Contains(published, "Keep me") {
		t.Fatalf("writings not published after migration: %s", published)
	}
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=PhraseMate Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=PhraseMate Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_TERMINAL_PROMPT=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
