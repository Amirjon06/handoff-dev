package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/session"
)

func TestVersionCommand(t *testing.T) {
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"version"}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != version {
		t.Fatalf("version output = %q, want %q", got, version)
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"nope"}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for unknown command")
	}
}

func TestRestorePrintsPlan(t *testing.T) {
	repoRoot := initGitRepo(t)
	path := writeTestSession(t, repoRoot)

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", path}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Restore plan",
		"Git root: " + repoRoot,
		"Git remote: https://github.com/Amirjon06/handoff-dev.git",
		"Git branch: main",
		"Git commit: faaf307bf4fa86c316586804bf88f3096511aabd",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("restore output missing %q:\n%s", want, got)
		}
	}
}

func TestRestoreRejectsMissingGitRoot(t *testing.T) {
	path := writeTestSession(t, t.TempDir())

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", path}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for non-git root")
	}

	if !strings.Contains(err.Error(), "verify git root") {
		t.Fatalf("error = %q, want git root verification error", err.Error())
	}
}

func TestRestoreRequiresSessionFile(t *testing.T) {
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"restore"}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error without session file")
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init returned error: %v\n%s", err, output)
	}

	return dir
}

func writeTestSession(t *testing.T, root string) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "session-*.json")
	if err != nil {
		t.Fatalf("CreateTemp returned error: %v", err)
	}
	defer file.Close()

	captured := session.New("test-machine", gitstate.State{
		Root:   root,
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: "faaf307bf4fa86c316586804bf88f3096511aabd",
	}, time.Date(2026, 8, 10, 18, 30, 0, 0, time.UTC))

	if err := session.WriteJSON(file, captured); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	return file.Name()
}
