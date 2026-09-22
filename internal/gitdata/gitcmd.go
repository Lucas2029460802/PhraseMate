package gitdata

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	localGitTimeout = 30 * time.Second
	fetchTimeout    = 8 * time.Second
	pushTimeout     = 40 * time.Second
)

type exitError struct {
	msg  string
	code int
}

func (e *exitError) Error() string { return e.msg }

func exitCode(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return -1
}

func (s *Sync) gitEnv() []string {
	drop := map[string]struct{}{
		"GIT_AUTHOR_NAME":     {},
		"GIT_AUTHOR_EMAIL":    {},
		"GIT_COMMITTER_NAME":  {},
		"GIT_COMMITTER_EMAIL": {},
		"GIT_TERMINAL_PROMPT": {},
		"GCM_INTERACTIVE":     {},
		"GIT_PAGER":           {},
		"PAGER":               {},
	}
	env := make([]string, 0, len(os.Environ())+6)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, skip := drop[key]; skip {
			continue
		}
		env = append(env, item)
	}
	return append(env,
		"GIT_AUTHOR_NAME="+s.author,
		"GIT_AUTHOR_EMAIL="+s.email,
		"GIT_COMMITTER_NAME="+s.author,
		"GIT_COMMITTER_EMAIL="+s.email,
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
		"GIT_PAGER=cat",
		"PAGER=cat",
	)
}

func (s *Sync) gitLocal(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), localGitTimeout)
	defer cancel()
	return s.run(ctx, nil, args...)
}

func (s *Sync) gitNet(timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.run(ctx, nil, args...)
}

func (s *Sync) gitStdin(input []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), localGitTimeout)
	defer cancel()
	return s.run(ctx, bytes.NewReader(input), args...)
}

func (s *Sync) run(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = s.root
	cmd.Env = s.gitEnv()
	cmd.WaitDelay = 2 * time.Second
	configureCmd(cmd)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, &exitError{msg: fmt.Sprintf("git %s 超时", strings.Join(args, " ")), code: -1}
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if len(msg) > 400 {
			msg = msg[:400]
		}
		code := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), &exitError{msg: "git " + strings.Join(args, " ") + ": " + msg, code: code}
	}
	return stdout.Bytes(), nil
}
