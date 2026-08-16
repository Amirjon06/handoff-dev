package terminalstate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCaptureRecordsRelativeWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "cmd", "relay")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	state, err := Capture(root, cwd, time.Date(2026, 8, 15, 21, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}

	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", state.SchemaVersion, SchemaVersion)
	}
	if state.CapturedAt != "2026-08-15T21:00:00Z" {
		t.Fatalf("captured_at = %q", state.CapturedAt)
	}
	if len(state.WorkingDirectories) != 1 || state.WorkingDirectories[0].Path != "cmd/relay" {
		t.Fatalf("working directories = %#v", state.WorkingDirectories)
	}
}

func TestCaptureRejectsDirectoryOutsideWorkspace(t *testing.T) {
	_, err := Capture(t.TempDir(), t.TempDir(), time.Now())
	if err == nil {
		t.Fatal("Capture returned nil error for outside directory")
	}
	if !strings.Contains(err.Error(), "is outside workspace") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestWorkspaceRoundTrip(t *testing.T) {
	root := t.TempDir()
	state := State{
		SchemaVersion: SchemaVersion,
		CapturedAt:    "2026-08-15T21:00:00Z",
		WorkingDirectories: []Directory{
			{Path: "."},
			{Path: "cmd/relay"},
		},
	}

	if err := WriteWorkspace(root, &state); err != nil {
		t.Fatalf("WriteWorkspace returned error: %v", err)
	}
	got, err := ReadWorkspace(root)
	if err != nil {
		t.Fatalf("ReadWorkspace returned error: %v", err)
	}
	if got == nil {
		t.Fatal("ReadWorkspace returned nil state")
	}
	if len(got.WorkingDirectories) != 2 {
		t.Fatalf("working directory count = %d, want 2", len(got.WorkingDirectories))
	}
}

func TestResolveDirectoriesReturnsAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, "cmd", "relay")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	state := &State{
		SchemaVersion: SchemaVersion,
		CapturedAt:    "2026-08-15T21:00:00Z",
		WorkingDirectories: []Directory{
			{Path: "."},
			{Path: "cmd/relay"},
		},
	}

	dirs, err := ResolveDirectories(root, state)
	if err != nil {
		t.Fatalf("ResolveDirectories returned error: %v", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks root returned error: %v", err)
	}
	cmdDir, err = filepath.EvalSymlinks(cmdDir)
	if err != nil {
		t.Fatalf("EvalSymlinks cmdDir returned error: %v", err)
	}
	if len(dirs) != 2 {
		t.Fatalf("dir count = %d, want 2", len(dirs))
	}
	if dirs[0] != root {
		t.Fatalf("first dir = %q, want %q", dirs[0], root)
	}
	if dirs[1] != cmdDir {
		t.Fatalf("second dir = %q, want %q", dirs[1], cmdDir)
	}
}

func TestResolveDirectoriesRejectsSymlinkOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Skipf("Symlink returned error: %v", err)
	}
	state := &State{
		SchemaVersion: SchemaVersion,
		CapturedAt:    "2026-08-15T21:00:00Z",
		WorkingDirectories: []Directory{
			{Path: "outside"},
		},
	}

	_, err := ResolveDirectories(root, state)
	if err == nil {
		t.Fatal("ResolveDirectories returned nil error for symlink outside workspace")
	}
	if !strings.Contains(err.Error(), "resolves outside workspace") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestReadWorkspaceReturnsNilWhenMissing(t *testing.T) {
	state, err := ReadWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("ReadWorkspace returned error: %v", err)
	}
	if state != nil {
		t.Fatalf("state = %#v, want nil", state)
	}
}

func TestReadJSONRejectsUnsafePath(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-15T21:00:00Z",
		"working_directories": [
			{"path": "../outside"}
		]
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error for unsafe path")
	}
	if got := err.Error(); got != `terminal state working_directories[0].path "../outside" is unsafe` {
		t.Fatalf("error = %q", got)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := State{
		SchemaVersion: SchemaVersion,
		CapturedAt:    "2026-08-15T21:00:00Z",
		WorkingDirectories: []Directory{
			{Path: "."},
		},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, original); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
	got, err := ReadJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if got.WorkingDirectories[0].Path != "." {
		t.Fatalf("path = %q", got.WorkingDirectories[0].Path)
	}
}
