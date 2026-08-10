package session

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/gitstate"
)

func TestSessionJSONRoundTrip(t *testing.T) {
	capturedAt := time.Date(2026, 8, 10, 18, 30, 0, 0, time.UTC)
	original := New("amir-macbook", gitstate.State{
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
