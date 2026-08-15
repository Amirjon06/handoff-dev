package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/editorstate"
	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/session"
	"github.com/Amirjon06/handoff-dev/internal/transport"
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

func TestPingCommandChecksListener(t *testing.T) {
	oldPingListener := pingListener
	t.Cleanup(func() { pingListener = oldPingListener })
	pingListener = func(ctx context.Context, target string) (transport.Health, error) {
		if target != "http://127.0.0.1:8765" {
			t.Fatalf("target = %q", target)
		}
		return transport.Health{Status: "ok", Service: "staterelay"}, nil
	}

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"ping", "--to", "http://127.0.0.1:8765"}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Listener: http://127.0.0.1:8765",
		"Status: ok",
		"Service: staterelay",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestPingRequiresTarget(t *testing.T) {
	var stdout bytes.Buffer
	err := run(context.Background(), []string{"ping"}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error without target")
	}
	if err.Error() != "ping requires --to" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestSendCommandPostsSession(t *testing.T) {
	path := writeTestSession(t, t.TempDir(), "faaf307bf4fa86c316586804bf88f3096511aabd")
	var received session.Session
	oldSendSession := sendSession
	t.Cleanup(func() { sendSession = oldSendSession })
	sendSession = func(ctx context.Context, target string, captured session.Session) (transport.Receipt, error) {
		if target != "http://127.0.0.1:8765" {
			t.Fatalf("target = %q", target)
		}
		received = captured
		return transport.Receipt{ID: "session-1", Message: "session stored"}, nil
	}

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"send", "--to", "http://127.0.0.1:8765", path}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if received.Git.Commit != "faaf307bf4fa86c316586804bf88f3096511aabd" {
		t.Fatalf("commit = %q", received.Git.Commit)
	}
	got := stdout.String()
	for _, want := range []string{
		"Sent session to http://127.0.0.1:8765",
		"Receipt: session-1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestSendRequiresTarget(t *testing.T) {
	var stdout bytes.Buffer
	err := run(context.Background(), []string{"send", "session.json"}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error without target")
	}
	if err.Error() != "send requires --to" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestInboxListsReceivedSessions(t *testing.T) {
	inbox := t.TempDir()
	path := filepath.Join(inbox, "received.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	captured := session.New("test-machine", gitstate.State{
		Root:   "/repo/staterelay",
		Name:   "staterelay",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: "0c8cc07845ffae5a9f5e479f4b094b30a9a7a691",
		Dirty:  true,
		ChangedFiles: []gitstate.ChangedFile{
			{Path: "README.md", Status: "modified"},
		},
	}, time.Date(2026, 8, 15, 20, 15, 41, 0, time.UTC))
	captured.Editor = testEditorState("/repo/staterelay")
	if err := session.WriteJSON(file, captured); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	var stdout bytes.Buffer
	err = run(context.Background(), []string{"inbox", "--inbox", inbox}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Inbox: " + inbox,
		path,
		"  repo: staterelay",
		"  branch: main",
		"  commit: 0c8cc07845ff",
		"  dirty: true",
		"  changed files: 1",
		"  editor files: 1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("inbox output missing %q:\n%s", want, got)
		}
	}
}

func TestInboxPrintsEmptyState(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"inbox", "--inbox", inbox}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Inbox: " + inbox,
		"No received sessions",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("inbox output missing %q:\n%s", want, got)
		}
	}
}

func TestCaptureJSONIncludesChangedFiles(t *testing.T) {
	repoRoot, _ := initGitRepo(t)
	if err := os.WriteFile(repoRoot+"/README.md", []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(repoRoot+"/notes.txt", []byte("scratch\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"capture", "--path", repoRoot, "--json"}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	captured, err := session.ReadJSON(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}

	if !captured.Git.Dirty {
		t.Fatal("dirty = false, want true")
	}
	if captured.Git.Name != filepath.Base(repoRoot) {
		t.Fatalf("git name = %q, want %q", captured.Git.Name, filepath.Base(repoRoot))
	}
	if len(captured.Git.ChangedFiles) != 2 {
		t.Fatalf("changed file count = %d, want 2", len(captured.Git.ChangedFiles))
	}
	assertChangedFile(t, captured.Git.ChangedFiles, "README.md", "modified")
	assertChangedFile(t, captured.Git.ChangedFiles, "notes.txt", "untracked")
}

func TestCaptureJSONIncludesEditorState(t *testing.T) {
	repoRoot, _ := initGitRepo(t)
	writeTestEditorState(t, repoRoot)

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"capture", "--path", repoRoot, "--json"}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	captured, err := session.ReadJSON(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}

	if captured.Editor == nil {
		t.Fatal("editor state = nil")
	}
	if captured.Editor.ActiveFile == nil || *captured.Editor.ActiveFile != "README.md" {
		t.Fatalf("active file = %#v", captured.Editor.ActiveFile)
	}
	if len(captured.Editor.OpenFiles) != 1 {
		t.Fatalf("open file count = %d, want 1", len(captured.Editor.OpenFiles))
	}
	if captured.Editor.OpenFiles[0].Selections[0].Active.Line != 3 {
		t.Fatalf("active line = %d, want 3", captured.Editor.OpenFiles[0].Selections[0].Active.Line)
	}
}

func TestCapturePrintsSnapshotDetails(t *testing.T) {
	repoRoot, _ := initGitRepo(t)
	if err := os.WriteFile(repoRoot+"/README.md", []byte("# Changed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"capture", "--path", repoRoot}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Git dirty: true",
		"Changed file: modified README.md (10 bytes captured, sha256 fa8549bc791b)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("capture output missing %q:\n%s", want, got)
		}
	}
}

func TestRestorePrintsPlan(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	path := writeTestSessionWithEditor(t, repoRoot, gitstate.State{
		Root:   repoRoot,
		Name:   "handoff-dev",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
		Dirty:  true,
		ChangedFiles: []gitstate.ChangedFile{
			{
				Path:            "README.md",
				Status:          "modified",
				Size:            11,
				ContentCaptured: true,
				ContentEncoding: "utf-8",
				ContentSHA256:   "fa8549bc791b513f06435d4e2b912b37bfed2e8388ad5edd89c33a9fee467f7a",
				Content:         "# Changed\n",
			},
		},
	}, testEditorState(repoRoot))

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", path}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Restore plan",
		"Git root: " + repoRoot,
		"Git name: handoff-dev",
		"Git remote: https://github.com/Amirjon06/handoff-dev.git",
		"Git branch: main",
		"Git commit: " + commit,
		"Git dirty: true",
		"Changed files:",
		"- modified README.md (11 bytes captured, sha256 fa8549bc791b)",
		"Editor state: 1 open file(s)",
		"Active editor file: README.md",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("restore output missing %q:\n%s", want, got)
		}
	}
}

func TestRestoreAcceptsPathOverride(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	destinationRoot := filepath.Join(t.TempDir(), "destination")
	runGit(t, t.TempDir(), "clone", repoRoot, destinationRoot)
	verifiedDestinationRoot := strings.TrimSpace(runGit(t, destinationRoot, "rev-parse", "--show-toplevel"))
	path := writeTestSessionWithGit(t, repoRoot, gitstate.State{
		Root:   repoRoot,
		Name:   "handoff-dev",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
	})

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", "--path", destinationRoot, path}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Restore plan",
		"Git root: " + repoRoot,
		"Restore root: " + verifiedDestinationRoot,
		"Git branch: main",
		"Git commit: " + commit,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("restore output missing %q:\n%s", want, got)
		}
	}
}

func TestRestoreUsesLatestInboxSession(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	inbox := t.TempDir()
	older := session.New("test-machine", gitstate.State{
		Root:   "/source/older",
		Name:   "older",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
	}, time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC))
	newer := session.New("test-machine", gitstate.State{
		Root:   "/source/newer",
		Name:   "staterelay",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
	}, time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	writeSessionFileAt(t, filepath.Join(inbox, "older.json"), older)
	writeSessionFileAt(t, filepath.Join(inbox, "newer.json"), newer)

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", "--path", repoRoot, "--inbox", inbox, "latest"}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{
		"Session file: " + filepath.Join(inbox, "newer.json"),
		"Git root: /source/newer",
		"Git name: staterelay",
		"Git commit: " + commit,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("restore output missing %q:\n%s", want, got)
		}
	}
}

func TestRestoreUsesNamedInboxSession(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	inbox := t.TempDir()
	captured := session.New("test-machine", gitstate.State{
		Root:   "/source/named",
		Name:   "staterelay",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
	}, time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC))
	writeSessionFileAt(t, filepath.Join(inbox, "received.json"), captured)

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", "--path", repoRoot, "--inbox", inbox, "received"}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Session file: "+filepath.Join(inbox, "received.json")) {
		t.Fatalf("restore output missing named inbox path:\n%s", got)
	}
}

func TestRestoreRejectsEmptyInbox(t *testing.T) {
	repoRoot, _ := initGitRepo(t)
	inbox := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"restore", "--path", repoRoot, "--inbox", inbox, "latest"}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for empty inbox")
	}
	if err.Error() != "no received sessions in "+inbox {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestRestoreRejectsMissingGitRoot(t *testing.T) {
	path := writeTestSession(t, t.TempDir(), "faaf307bf4fa86c316586804bf88f3096511aabd")

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", path}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for non-git root")
	}

	if !strings.Contains(err.Error(), "verify git root") {
		t.Fatalf("error = %q, want git root verification error", err.Error())
	}
}

func TestRestoreRejectsBranchMismatch(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	path := writeTestSessionWithGit(t, repoRoot, gitstate.State{
		Root:   repoRoot,
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "feature/missing",
		Commit: commit,
	})

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", path}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for branch mismatch")
	}

	if !strings.Contains(err.Error(), "session branch feature/missing does not match current branch") {
		t.Fatalf("error = %q, want branch mismatch error", err.Error())
	}
}

func TestRestoreRejectsCommitMismatch(t *testing.T) {
	repoRoot, _ := initGitRepo(t)
	path := writeTestSessionWithGit(t, repoRoot, gitstate.State{
		Root:   repoRoot,
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: "0000000000000000000000000000000000000000",
	})

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", path}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for commit mismatch")
	}

	if !strings.Contains(err.Error(), "session commit 0000000000000000000000000000000000000000 does not match current commit") {
		t.Fatalf("error = %q, want commit mismatch error", err.Error())
	}
}

func TestRestoreApplyWritesCapturedFiles(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	verifiedRoot := strings.TrimSpace(runGit(t, repoRoot, "rev-parse", "--show-toplevel"))
	path := writeTestSessionWithEditor(t, repoRoot, gitstate.State{
		Root:   repoRoot,
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
		Dirty:  true,
		ChangedFiles: []gitstate.ChangedFile{
			{
				Path:            "README.md",
				Status:          "modified",
				Size:            10,
				ContentCaptured: true,
				ContentEncoding: "utf-8",
				ContentSHA256:   "359f6acb2ccacf06e0aa9b8951561b13e71c7daed3b210e0d8d413705641eadd",
				Content:         "# Applied\n",
			},
			{
				Path:   "notes.txt",
				Status: "modified",
			},
		},
	}, testEditorState("/source/repo"))

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", "--apply", path}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	content, err := os.ReadFile(repoRoot + "/README.md")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "# Applied\n" {
		t.Fatalf("README content = %q", content)
	}
	editor, err := editorstate.ReadWorkspace(repoRoot)
	if err != nil {
		t.Fatalf("ReadWorkspace returned error: %v", err)
	}
	if editor == nil {
		t.Fatal("editor state = nil")
	}
	if editor.WorkspaceFolder == nil || *editor.WorkspaceFolder != verifiedRoot {
		t.Fatalf("workspace folder = %#v, want %q", editor.WorkspaceFolder, verifiedRoot)
	}
	got := stdout.String()
	for _, want := range []string{
		"Applied 1 changed file(s)",
		"Skipped 1 changed file(s) without captured content: modified notes.txt",
		"Restored editor state: 1 open file(s)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestRestoreApplyDryRunDoesNotWriteFiles(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	path := writeTestSessionWithEditor(t, repoRoot, gitstate.State{
		Root:   repoRoot,
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
		Dirty:  true,
		ChangedFiles: []gitstate.ChangedFile{
			{
				Path:            "README.md",
				Status:          "modified",
				Size:            10,
				ContentCaptured: true,
				ContentEncoding: "utf-8",
				ContentSHA256:   "359f6acb2ccacf06e0aa9b8951561b13e71c7daed3b210e0d8d413705641eadd",
				Content:         "# Applied\n",
			},
			{
				Path:   "notes.txt",
				Status: "modified",
			},
		},
	}, testEditorState("/source/repo"))

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", "--apply", "--dry-run", path}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	content, err := os.ReadFile(repoRoot + "/README.md")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "# Test Repo\n" {
		t.Fatalf("README content = %q", content)
	}
	editor, err := editorstate.ReadWorkspace(repoRoot)
	if err != nil {
		t.Fatalf("ReadWorkspace returned error: %v", err)
	}
	if editor != nil {
		t.Fatalf("editor state = %#v, want nil", editor)
	}
	got := stdout.String()
	for _, want := range []string{
		"Would apply 1 changed file(s)",
		"Skipped 1 changed file(s) without captured content: modified notes.txt",
		"Would restore editor state: 1 open file(s)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestRestoreApplyRejectsContentHashMismatch(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	path := writeTestSessionWithGit(t, repoRoot, gitstate.State{
		Root:   repoRoot,
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
		Dirty:  true,
		ChangedFiles: []gitstate.ChangedFile{
			{
				Path:            "README.md",
				Status:          "modified",
				Size:            10,
				ContentCaptured: true,
				ContentEncoding: "utf-8",
				ContentSHA256:   "0000000000000000000000000000000000000000000000000000000000000000",
				Content:         "# Applied\n",
			},
		},
	})

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", "--apply", path}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for content hash mismatch")
	}
	if !strings.Contains(err.Error(), "changed file README.md content hash mismatch") {
		t.Fatalf("error = %q", err.Error())
	}

	content, err := os.ReadFile(repoRoot + "/README.md")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "# Test Repo\n" {
		t.Fatalf("README content = %q", content)
	}
}

func TestRestoreApplyRejectsDirtyWorkingTree(t *testing.T) {
	repoRoot, commit := initGitRepo(t)
	if err := os.WriteFile(repoRoot+"/README.md", []byte("# Local change\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	path := writeTestSessionWithGit(t, repoRoot, gitstate.State{
		Root:   repoRoot,
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
		Dirty:  true,
		ChangedFiles: []gitstate.ChangedFile{
			{
				Path:            "README.md",
				Status:          "modified",
				Size:            10,
				ContentCaptured: true,
				ContentEncoding: "utf-8",
				ContentSHA256:   "359f6acb2ccacf06e0aa9b8951561b13e71c7daed3b210e0d8d413705641eadd",
				Content:         "# Applied\n",
			},
		},
	})

	var stdout bytes.Buffer
	err := run(context.Background(), []string{"restore", "--apply", path}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for dirty working tree")
	}
	if !strings.Contains(err.Error(), "refusing to apply over dirty working tree") {
		t.Fatalf("error = %q", err.Error())
	}
	if !strings.Contains(err.Error(), "modified README.md") {
		t.Fatalf("error = %q, want dirty file details", err.Error())
	}
}

func TestRestoreRequiresSessionFile(t *testing.T) {
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"restore"}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error without session file")
	}
}

func initGitRepo(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "--initial-branch=main")
	runGit(t, dir, "config", "user.name", "StateRelay Tests")
	runGit(t, dir, "config", "user.email", "tests@staterelay.local")
	runGit(t, dir, "remote", "add", "origin", "https://github.com/Amirjon06/handoff-dev.git")

	if err := os.WriteFile(dir+"/README.md", []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "Initial commit")
	commit := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))

	return dir, commit
}

func assertChangedFile(t *testing.T, files []gitstate.ChangedFile, path string, status string) {
	t.Helper()

	for _, file := range files {
		if file.Path == path && file.Status == status {
			return
		}
	}

	t.Fatalf("changed files %#v missing %s %s", files, status, path)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v returned error: %v\n%s", args, err, output)
	}

	return string(output)
}

func writeTestSession(t *testing.T, root string, commit string) string {
	t.Helper()

	return writeTestSessionWithGit(t, root, gitstate.State{
		Root:   root,
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: commit,
	})
}

func writeTestSessionWithGit(t *testing.T, root string, git gitstate.State) string {
	t.Helper()

	return writeTestSessionWithEditor(t, root, git, nil)
}

func writeTestSessionWithEditor(t *testing.T, root string, git gitstate.State, editor *editorstate.State) string {
	t.Helper()

	if git.Name == "" {
		git.Name = filepath.Base(root)
	}

	file, err := os.CreateTemp(t.TempDir(), "session-*.json")
	if err != nil {
		t.Fatalf("CreateTemp returned error: %v", err)
	}
	defer file.Close()

	captured := session.NewWithEditor("test-machine", git, editor, time.Date(2026, 8, 10, 18, 30, 0, 0, time.UTC))

	if err := session.WriteJSON(file, captured); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	return file.Name()
}

func writeSessionFileAt(t *testing.T, path string, captured session.Session) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	defer file.Close()
	if err := session.WriteJSON(file, captured); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
}

func writeTestEditorState(t *testing.T, root string) {
	t.Helper()

	if err := editorstate.WriteWorkspace(root, testEditorState(root)); err != nil {
		t.Fatalf("WriteWorkspace returned error: %v", err)
	}
}

func testEditorState(root string) *editorstate.State {
	activeFile := "README.md"
	return &editorstate.State{
		SchemaVersion:   editorstate.SchemaVersion,
		CapturedAt:      "2026-08-15T18:30:00Z",
		WorkspaceFolder: &root,
		ActiveFile:      &activeFile,
		OpenFiles: []editorstate.File{
			{
				Path:       "README.md",
				LanguageID: "markdown",
				IsDirty:    true,
				Selections: []editorstate.Selection{
					{
						Anchor: editorstate.Position{Line: 3, Character: 0},
						Active: editorstate.Position{Line: 3, Character: 8},
					},
				},
			},
		},
	}
}
