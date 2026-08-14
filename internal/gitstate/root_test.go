package gitstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
			expectedRoot + " [status --porcelain=v1]":          " M README.md\n?? notes.txt",
		},
	}

	got, err := capture(context.Background(), runner, "")
	if err != nil {
		t.Fatalf("capture returned error: %v", err)
	}

	if got.Root != expectedRoot {
		t.Fatalf("root = %q, want %q", got.Root, expectedRoot)
	}
	if got.Name != filepath.Base(expectedRoot) {
		t.Fatalf("name = %q, want %q", got.Name, filepath.Base(expectedRoot))
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
	if !got.Dirty {
		t.Fatal("dirty = false, want true")
	}
	if len(got.ChangedFiles) != 2 {
		t.Fatalf("changed file count = %d, want 2", len(got.ChangedFiles))
	}
}

func TestChangesReturnsCleanWorkingTree(t *testing.T) {
	runner := &fakeRunner{output: ""}

	got, err := changes(context.Background(), runner, "/repo")
	if err != nil {
		t.Fatalf("changes returned error: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("changed files = %#v, want none", got)
	}
}

func TestParseStatus(t *testing.T) {
	got := parseStatus(" M README.md\nA  main.go\nR  old.go -> new.go\n?? scratch.txt")
	want := []ChangedFile{
		{Path: "README.md", Status: "modified"},
		{Path: "main.go", Status: "added"},
		{Path: "new.go", Status: "renamed"},
		{Path: "scratch.txt", Status: "untracked"},
	}

	if len(got) != len(want) {
		t.Fatalf("changed file count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("changed file %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestCaptureFileSnapshotsCapturesTextFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/README.md", []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	files, err := captureFileSnapshots(root, []ChangedFile{
		{Path: "README.md", Status: "modified"},
	})
	if err != nil {
		t.Fatalf("captureFileSnapshots returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if !files[0].ContentCaptured {
		t.Fatal("content captured = false, want true")
	}
	if files[0].ContentEncoding != "utf-8" {
		t.Fatalf("content encoding = %q, want utf-8", files[0].ContentEncoding)
	}
	if files[0].ContentSHA256 != "fa8549bc791b513f06435d4e2b912b37bfed2e8388ad5edd89c33a9fee467f7a" {
		t.Fatalf("content sha256 = %q", files[0].ContentSHA256)
	}
	if files[0].Content != "# Changed\n" {
		t.Fatalf("content = %q", files[0].Content)
	}
}

func TestCaptureFileSnapshotsSkipsBinaryFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/image.bin", []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	files, err := captureFileSnapshots(root, []ChangedFile{
		{Path: "image.bin", Status: "modified"},
	})
	if err != nil {
		t.Fatalf("captureFileSnapshots returned error: %v", err)
	}

	if files[0].ContentCaptured {
		t.Fatal("content captured = true, want false")
	}
}

func TestCaptureFileSnapshotsSkipsUnsafePaths(t *testing.T) {
	files, err := captureFileSnapshots(t.TempDir(), []ChangedFile{
		{Path: "../secret.txt", Status: "modified"},
	})
	if err != nil {
		t.Fatalf("captureFileSnapshots returned error: %v", err)
	}

	if files[0].ContentCaptured {
		t.Fatal("content captured = true, want false")
	}
}
