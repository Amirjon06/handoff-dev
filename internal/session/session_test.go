package session

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/browserstate"
	"github.com/Amirjon06/handoff-dev/internal/deviceidentity"
	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/terminalstate"
)

func TestSessionJSONRoundTrip(t *testing.T) {
	capturedAt := time.Date(2026, 8, 10, 18, 30, 0, 0, time.UTC)
	original := New("amir-macbook", gitstate.State{
		Name:   "handoff-dev",
		Root:   "/Users/amir/projects/handoff-dev",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: "faaf307bf4fa86c316586804bf88f3096511aabd",
	}, capturedAt)

	var buf bytes.Buffer
	if err := WriteJSON(&buf, original); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	got, err := ReadJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}

	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", got.SchemaVersion, SchemaVersion)
	}
	if got.Source.Hostname != "amir-macbook" {
		t.Fatalf("hostname = %q, want amir-macbook", got.Source.Hostname)
	}
	if got.Git.Remote != "https://github.com/Amirjon06/handoff-dev.git" {
		t.Fatalf("remote = %q", got.Git.Remote)
	}
}

func TestSessionJSONRoundTripWithTerminalState(t *testing.T) {
	capturedAt := time.Date(2026, 8, 15, 21, 0, 0, 0, time.UTC)
	original := NewWithWorkspace("amir-macbook", gitstate.State{
		Name:   "handoff-dev",
		Root:   "/Users/amir/projects/handoff-dev",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: "faaf307bf4fa86c316586804bf88f3096511aabd",
	}, nil, &terminalstate.State{
		SchemaVersion: terminalstate.SchemaVersion,
		CapturedAt:    "2026-08-15T21:00:00Z",
		WorkingDirectories: []terminalstate.Directory{
			{Path: "."},
		},
	}, nil, capturedAt)

	var buf bytes.Buffer
	if err := WriteJSON(&buf, original); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	got, err := ReadJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if got.Terminal == nil {
		t.Fatal("terminal state = nil")
	}
	if got.Terminal.WorkingDirectories[0].Path != "." {
		t.Fatalf("terminal path = %q", got.Terminal.WorkingDirectories[0].Path)
	}
}

func TestSessionJSONRoundTripWithBrowserState(t *testing.T) {
	capturedAt := time.Date(2026, 8, 15, 22, 0, 0, 0, time.UTC)
	original := NewWithWorkspace("amir-macbook", gitstate.State{
		Name:   "handoff-dev",
		Root:   "/Users/amir/projects/handoff-dev",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: "faaf307bf4fa86c316586804bf88f3096511aabd",
	}, nil, nil, &browserstate.State{
		SchemaVersion: browserstate.SchemaVersion,
		CapturedAt:    "2026-08-15T22:00:00Z",
		Tabs: []browserstate.Tab{
			{URL: "https://go.dev/doc/"},
		},
	}, capturedAt)

	var buf bytes.Buffer
	if err := WriteJSON(&buf, original); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	got, err := ReadJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if got.Browser == nil {
		t.Fatal("browser state = nil")
	}
	if got.Browser.Tabs[0].URL != "https://go.dev/doc/" {
		t.Fatalf("browser url = %q", got.Browser.Tabs[0].URL)
	}
}

func TestSignedSessionJSONRoundTrip(t *testing.T) {
	identity, err := deviceidentity.New("amir-macbook", time.Date(2026, 8, 15, 23, 0, 0, 0, time.UTC), bytes.NewReader(bytes.Repeat([]byte{8}, 32)))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	original := New("amir-macbook", testGitState(), time.Date(2026, 8, 10, 18, 30, 0, 0, time.UTC))
	signed, err := Sign(original, identity)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, signed); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
	got, err := ReadJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}

	if got.Signature == nil {
		t.Fatal("signature = nil")
	}
	if got.Signature.DeviceName != "amir-macbook" {
		t.Fatalf("device name = %q", got.Signature.DeviceName)
	}
	if got.Signature.Fingerprint != identity.Fingerprint {
		t.Fatalf("fingerprint = %q, want %q", got.Signature.Fingerprint, identity.Fingerprint)
	}
}

func TestReadJSONRejectsTamperedSignedSession(t *testing.T) {
	identity, err := deviceidentity.New("amir-macbook", time.Date(2026, 8, 15, 23, 0, 0, 0, time.UTC), bytes.NewReader(bytes.Repeat([]byte{8}, 32)))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	signed, err := Sign(New("amir-macbook", testGitState(), time.Date(2026, 8, 10, 18, 30, 0, 0, time.UTC)), identity)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	signed.Git.Branch = "feature/tampered"

	var buf bytes.Buffer
	if err := WriteJSON(&buf, signed); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
	_, err = ReadJSON(strings.NewReader(buf.String()))
	if err == nil {
		t.Fatal("ReadJSON returned nil error for tampered signature")
	}
	if got := err.Error(); got != "session signature: session signature verification failed" {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsUnsafeTerminalPath(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-10T18:30:00Z",
		"source": {
			"hostname": "amir-macbook",
			"os": "darwin",
			"arch": "arm64"
		},
		"git": {
			"root": "/Users/amir/projects/handoff-dev",
			"name": "handoff-dev",
			"remote": "https://github.com/Amirjon06/handoff-dev.git",
			"branch": "main",
			"commit": "faaf307bf4fa86c316586804bf88f3096511aabd"
		},
		"terminal": {
			"schema_version": 1,
			"captured_at": "2026-08-15T21:00:00Z",
			"working_directories": [
				{"path": "../outside"}
			]
		}
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error for unsafe terminal path")
	}
	if got := err.Error(); got != `session terminal: terminal state working_directories[0].path "../outside" is unsafe` {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsInvalidBrowserURL(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-10T18:30:00Z",
		"source": {
			"hostname": "amir-macbook",
			"os": "darwin",
			"arch": "arm64"
		},
		"git": {
			"root": "/Users/amir/projects/handoff-dev",
			"name": "handoff-dev",
			"remote": "https://github.com/Amirjon06/handoff-dev.git",
			"branch": "main",
			"commit": "faaf307bf4fa86c316586804bf88f3096511aabd"
		},
		"browser": {
			"schema_version": 1,
			"captured_at": "2026-08-15T22:00:00Z",
			"tabs": [
				{"url": "ftp://example.com/file"}
			]
		}
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error for invalid browser URL")
	}
	if got := err.Error(); got != `session browser: browser state tabs[0].url: unsupported scheme "ftp"` {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsMissingCapturedAt(t *testing.T) {
	input := `{
		"schema_version": 1,
		"source": {
			"hostname": "amir-macbook",
			"os": "darwin",
			"arch": "arm64"
		},
		"git": {
			"root": "/Users/amir/projects/handoff-dev",
			"name": "handoff-dev",
			"remote": "https://github.com/Amirjon06/handoff-dev.git",
			"branch": "main",
			"commit": "faaf307bf4fa86c316586804bf88f3096511aabd"
		}
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error without captured_at")
	}

	if got := err.Error(); got != "session captured_at is required" {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsMissingGitName(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-10T18:30:00Z",
		"source": {
			"hostname": "amir-macbook",
			"os": "darwin",
			"arch": "arm64"
		},
		"git": {
			"root": "/Users/amir/projects/handoff-dev",
			"remote": "https://github.com/Amirjon06/handoff-dev.git",
			"branch": "main",
			"commit": "faaf307bf4fa86c316586804bf88f3096511aabd"
		}
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error without git name")
	}

	if got := err.Error(); got != "session git.name is required" {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsMissingChangedFilePath(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-10T18:30:00Z",
		"source": {
			"hostname": "amir-macbook",
			"os": "darwin",
			"arch": "arm64"
		},
		"git": {
			"root": "/Users/amir/projects/handoff-dev",
			"name": "handoff-dev",
			"remote": "https://github.com/Amirjon06/handoff-dev.git",
			"branch": "main",
			"commit": "faaf307bf4fa86c316586804bf88f3096511aabd",
			"changed_files": [
				{
					"status": "modified"
				}
			]
		}
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error without changed file path")
	}

	if got := err.Error(); got != "session git.changed_files[0].path is required" {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsMissingCapturedContentHash(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-10T18:30:00Z",
		"source": {
			"hostname": "amir-macbook",
			"os": "darwin",
			"arch": "arm64"
		},
		"git": {
			"root": "/Users/amir/projects/handoff-dev",
			"name": "handoff-dev",
			"remote": "https://github.com/Amirjon06/handoff-dev.git",
			"branch": "main",
			"commit": "faaf307bf4fa86c316586804bf88f3096511aabd",
			"changed_files": [
				{
					"path": "README.md",
					"status": "modified",
					"content_captured": true,
					"content_encoding": "utf-8",
					"content": "# Changed\n"
				}
			]
		}
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error without captured content hash")
	}

	if got := err.Error(); got != "session git.changed_files[0].content_sha256 is required" {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsUnsafeChangedFilePath(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-10T18:30:00Z",
		"source": {
			"hostname": "amir-macbook",
			"os": "darwin",
			"arch": "arm64"
		},
		"git": {
			"root": "/Users/amir/projects/handoff-dev",
			"name": "handoff-dev",
			"remote": "https://github.com/Amirjon06/handoff-dev.git",
			"branch": "main",
			"commit": "faaf307bf4fa86c316586804bf88f3096511aabd",
			"changed_files": [
				{
					"path": "../secret.txt",
					"status": "modified"
				}
			]
		}
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error for unsafe changed file path")
	}

	if got := err.Error(); got != `session git.changed_files[0].path "../secret.txt" is unsafe` {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsUnsupportedSchema(t *testing.T) {
	input := `{"schema_version": 99}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error for unsupported schema")
	}

	if got := err.Error(); got != "unsupported session schema version 99" {
		t.Fatalf("error = %q", got)
	}
}

func testGitState() gitstate.State {
	return gitstate.State{
		Name:   "handoff-dev",
		Root:   "/Users/amir/projects/handoff-dev",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: "faaf307bf4fa86c316586804bf88f3096511aabd",
	}
}
