package browserstate

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCaptureRecordsURLs(t *testing.T) {
	state, err := Capture([]string{
		"https://go.dev/doc/",
		"http://localhost:8765/health",
	}, time.Date(2026, 8, 15, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}

	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", state.SchemaVersion, SchemaVersion)
	}
	if state.CapturedAt != "2026-08-15T22:00:00Z" {
		t.Fatalf("captured_at = %q", state.CapturedAt)
	}
	if len(state.Tabs) != 2 {
		t.Fatalf("tab count = %d, want 2", len(state.Tabs))
	}
}

func TestCaptureRejectsMissingURLs(t *testing.T) {
	_, err := Capture(nil, time.Now())
	if err == nil {
		t.Fatal("Capture returned nil error without URLs")
	}
	if err.Error() != "at least one browser URL is required" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestCaptureRejectsUnsupportedScheme(t *testing.T) {
	_, err := Capture([]string{"file:///tmp/index.html"}, time.Now())
	if err == nil {
		t.Fatal("Capture returned nil error for unsupported scheme")
	}
	if err.Error() != `unsupported scheme "file"` {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestWorkspaceRoundTrip(t *testing.T) {
	root := t.TempDir()
	state := State{
		SchemaVersion: SchemaVersion,
		CapturedAt:    "2026-08-15T22:00:00Z",
		Tabs: []Tab{
			{URL: "https://go.dev/doc/"},
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
	if got.Tabs[0].URL != "https://go.dev/doc/" {
		t.Fatalf("url = %q", got.Tabs[0].URL)
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

func TestReadJSONRejectsInvalidURL(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-15T22:00:00Z",
		"tabs": [
			{"url": "ftp://example.com/file"}
		]
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error for invalid URL")
	}
	if got := err.Error(); got != `browser state tabs[0].url: unsupported scheme "ftp"` {
		t.Fatalf("error = %q", got)
	}
}

func TestReadJSONRejectsMissingTabs(t *testing.T) {
	input := `{
		"schema_version": 1,
		"captured_at": "2026-08-15T22:00:00Z"
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error without tabs")
	}
	if got := err.Error(); got != "browser state tabs are required" {
		t.Fatalf("error = %q", got)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := State{
		SchemaVersion: SchemaVersion,
		CapturedAt:    "2026-08-15T22:00:00Z",
		Tabs: []Tab{
			{URL: "https://go.dev/doc/"},
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
	if got.Tabs[0].URL != "https://go.dev/doc/" {
		t.Fatalf("url = %q", got.Tabs[0].URL)
	}
}
