package gitstate

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type fakeRunner struct {
	output string
	err    error
	calls  []string
}

func (f *fakeRunner) Run(_ context.Context, dir string, args ...string) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("%s %v", dir, args))
	return f.output, f.err
}

func TestRootReturnsAbsoluteGitRoot(t *testing.T) {
	expectedRoot := t.TempDir()
	runner := &fakeRunner{output: expectedRoot}

	got, err := root(context.Background(), runner, expectedRoot)
	if err != nil {
		t.Fatalf("root returned error: %v", err)
	}

	if got != expectedRoot {
		t.Fatalf("root = %q, want %q", got, expectedRoot)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("git call count = %d, want 1", len(runner.calls))
	}
}

func TestRootDefaultsToCurrentDirectory(t *testing.T) {
	runner := &fakeRunner{output: "/Users/amir/projects/handoff-dev"}

	_, err := root(context.Background(), runner, "")
	if err != nil {
		t.Fatalf("root returned error: %v", err)
	}

	if got := runner.calls[0]; got != ". [rev-parse --show-toplevel]" {
		t.Fatalf("git call = %q, want current directory rev-parse", got)
	}
}

func TestRootReturnsHelpfulErrorOutsideRepo(t *testing.T) {
	runner := &fakeRunner{err: errors.New("fatal: not a git repository")}

	_, err := root(context.Background(), runner, "/tmp")
	if err == nil {
		t.Fatal("root returned nil error outside git repository")
	}

	if got := err.Error(); got != "/tmp is not inside a git repository: fatal: not a git repository" {
		t.Fatalf("error = %q", got)
	}
}
