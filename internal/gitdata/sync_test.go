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
		Term:      "luminous",
		MeaningZH: "发光的",
		MeaningEN: "full of light",
		Phonetic:  "ˈluːmɪnəs",
	}); err != nil {
		t.Fatal(err)
	}

	if err := gs.Bootstrap(st); err != nil {
		t.Fatal(err)
	}
	if err := gs.Flush(); err != nil {
		t.Fatal(err)
	}

	body := gitRun(t, bare, "show", "data:words.json")
	if !strings.Contains(body, "luminous") || !strings.Contains(body, "发光的") {
		t.Fatalf("words.json = %s", body)
	}
	if strings.Contains(body, "sk-secret-test") {
		t.Fatal("API key leaked into data branch")
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
