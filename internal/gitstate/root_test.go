package gitstate

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type fakeRunner struct {
	output  string
	err     error
	calls   []string
	outputs map[string]string
	errors  map[string]error
}

func (f *fakeRunner) Run(_ context.Context, dir string, args ...string) (string, error) {
	call := fmt.Sprintf("%s %v", dir, args)
	f.calls = append(f.calls, call)
	if f.errors != nil {
		if err, ok := f.errors[call]; ok {
			return "", err
		}
	}
	if f.outputs != nil {
		if output, ok := f.outputs[call]; ok {
			return output, nil
		}
	}
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

func TestBranchReturnsCurrentBranch(t *testing.T) {
	runner := &fakeRunner{output: "main"}

	got, err := branch(context.Background(), runner, "/repo")
	if err != nil {
		t.Fatalf("branch returned error: %v", err)
	}

	if got != "main" {
		t.Fatalf("branch = %q, want main", got)
	}
	if got := runner.calls[0]; got != "/repo [branch --show-current]" {
		t.Fatalf("git call = %q, want branch command", got)
	}
}

func TestBranchReportsDetachedHead(t *testing.T) {
	runner := &fakeRunner{output: ""}

	_, err := branch(context.Background(), runner, "/repo")
	if err == nil {
		t.Fatal("branch returned nil error for detached HEAD")
	}

	if got := err.Error(); got != "repository is in detached HEAD state" {
		t.Fatalf("error = %q", got)
	}
}

func TestCommitReturnsCurrentCommit(t *testing.T) {
	runner := &fakeRunner{output: "24c5116f2f2d5f51e0f9a6b9f0f7670634dbabcd"}

	got, err := commit(context.Background(), runner, "/repo")
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}

	if got != "24c5116f2f2d5f51e0f9a6b9f0f7670634dbabcd" {
		t.Fatalf("commit = %q", got)
	}
	if got := runner.calls[0]; got != "/repo [rev-parse HEAD]" {
		t.Fatalf("git call = %q, want commit command", got)
	}
}

func TestCommitReturnsHelpfulError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("fatal: bad revision HEAD")}

	_, err := commit(context.Background(), runner, "/repo")
	if err == nil {
		t.Fatal("commit returned nil error")
	}

	if got := err.Error(); got != "read current git commit: fatal: bad revision HEAD" {
		t.Fatalf("error = %q", got)
	}
}

func TestRemoteReturnsOriginURL(t *testing.T) {
	runner := &fakeRunner{output: "https://github.com/Amirjon06/handoff-dev.git"}

	got, err := remote(context.Background(), runner, "/repo")
	if err != nil {
		t.Fatalf("remote returned error: %v", err)
	}

	if got != "https://github.com/Amirjon06/handoff-dev.git" {
		t.Fatalf("remote = %q", got)
	}
	if got := runner.calls[0]; got != "/repo [config --get remote.origin.url]" {
		t.Fatalf("git call = %q, want remote command", got)
	}
}

func TestRemoteReportsMissingOrigin(t *testing.T) {
	runner := &fakeRunner{output: ""}

	_, err := remote(context.Background(), runner, "/repo")
	if err == nil {
		t.Fatal("remote returned nil error for missing origin")
	}

	if got := err.Error(); got != "repository has no origin remote" {
		t.Fatalf("error = %q", got)
	}
}

func TestRemoteReturnsHelpfulError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("exit status 1")}

	_, err := remote(context.Background(), runner, "/repo")
	if err == nil {
		t.Fatal("remote returned nil error")
	}

	if got := err.Error(); got != "read git origin remote: exit status 1" {
		t.Fatalf("error = %q", got)
	}
}

func TestCaptureReturnsGitState(t *testing.T) {
	expectedRoot := t.TempDir()
	runner := &fakeRunner{
		outputs: map[string]string{
			". [rev-parse --show-toplevel]":                    expectedRoot,
			expectedRoot + " [config --get remote.origin.url]": "https://github.com/Amirjon06/handoff-dev.git",
			expectedRoot + " [branch --show-current]":          "main",
			expectedRoot + " [rev-parse HEAD]":                 "faaf307bf4fa86c316586804bf88f3096511aabd",
		},
	}

	got, err := capture(context.Background(), runner, "")
	if err != nil {
		t.Fatalf("capture returned error: %v", err)
	}

	if got.Root != expectedRoot {
		t.Fatalf("root = %q, want %q", got.Root, expectedRoot)
	}
	if got.Remote != "https://github.com/Amirjon06/handoff-dev.git" {
		t.Fatalf("remote = %q", got.Remote)
	}
	if got.Branch != "main" {
		t.Fatalf("branch = %q", got.Branch)
	}
	if got.Commit != "faaf307bf4fa86c316586804bf88f3096511aabd" {
		t.Fatalf("commit = %q", got.Commit)
	}
}
