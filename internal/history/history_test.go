package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/browserstate"
	"github.com/Amirjon06/handoff-dev/internal/editorstate"
	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/session"
	"github.com/Amirjon06/handoff-dev/internal/terminalstate"
)

func TestStoreRecordsAndListsReceivedSessions(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	captured := testSession()
	storedAt := time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC)
	event := NewReceivedEvent("session-1", "/tmp/session-1.json", captured, storedAt)
	if err := store.Record(context.Background(), event); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	events, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	got := events[0]
	if got.ID != "session-1" {
		t.Fatalf("id = %q", got.ID)
	}
	if got.Direction != DirectionReceived {
		t.Fatalf("direction = %q", got.Direction)
	}
	if got.RepoName != "staterelay" {
		t.Fatalf("repo name = %q", got.RepoName)
	}
	if got.ChangedFiles != 2 {
		t.Fatalf("changed files = %d", got.ChangedFiles)
	}
	if got.EditorFiles != 2 {
		t.Fatalf("editor files = %d", got.EditorFiles)
	}
	if got.TerminalDirs != 1 {
		t.Fatalf("terminal dirs = %d", got.TerminalDirs)
	}
	if got.BrowserTabs != 2 {
		t.Fatalf("browser tabs = %d", got.BrowserTabs)
	}
	if !got.StoredAt.Equal(storedAt) {
		t.Fatalf("stored at = %s", got.StoredAt)
	}
}

func TestStoreListsNewestFirst(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	older := NewReceivedEvent("older", "/tmp/older.json", testSession(), time.Date(2026, 8, 16, 18, 0, 0, 0, time.UTC))
	newer := NewReceivedEvent("newer", "/tmp/newer.json", testSession(), time.Date(2026, 8, 16, 19, 0, 0, 0, time.UTC))
	if err := store.Record(context.Background(), older); err != nil {
		t.Fatalf("Record older returned error: %v", err)
	}
	if err := store.Record(context.Background(), newer); err != nil {
		t.Fatalf("Record newer returned error: %v", err)
	}

	events, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if events[0].ID != "newer" {
		t.Fatalf("first event = %q", events[0].ID)
	}
	if events[1].ID != "older" {
		t.Fatalf("second event = %q", events[1].ID)
	}
}

func TestStoreCountReturnsSessionCount(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	if err := store.Record(context.Background(), NewReceivedEvent("session-1", "/tmp/session-1.json", testSession(), time.Now())); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	count, err := store.Count(context.Background())
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestStoreGetsReceivedSessionByID(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	event := NewReceivedEvent("session-1", "/tmp/session-1.json", testSession(), time.Now())
	if err := store.Record(context.Background(), event); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	got, ok, err := store.Get(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("Get did not find session-1")
	}
	if got.ID != "session-1" {
		t.Fatalf("id = %q", got.ID)
	}
	if got.SessionPath != "/tmp/session-1.json" {
		t.Fatalf("session path = %q", got.SessionPath)
	}
}

func TestStoreGetReturnsFalseForMissingSession(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	_, ok, err := store.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("Get found missing session")
	}
}

func testSession() session.Session {
	activeFile := "main.go"
	return session.Session{
		SchemaVersion: session.SchemaVersion,
		CapturedAt:    time.Date(2026, 8, 16, 17, 0, 0, 0, time.UTC),
		Source: session.Source{
			Hostname: "macbook",
			OS:       "darwin",
			Arch:     "arm64",
		},
		Git: gitstate.State{
			Root:   "/repo/staterelay",
			Name:   "staterelay",
			Remote: "https://github.com/Amirjon06/handoff-dev.git",
			Branch: "main",
			Commit: "91b472d7d36b0e62480258ee923fcd50e4cce10d",
			Dirty:  true,
			ChangedFiles: []gitstate.ChangedFile{
				{Path: "main.go", Status: "M"},
				{Path: "notes.txt", Status: "??"},
			},
		},
		Editor: &editorstate.State{
			SchemaVersion: editorstate.SchemaVersion,
			CapturedAt:    "2026-08-16T17:00:00Z",
			OpenFiles: []editorstate.File{
				{Path: "main.go", LanguageID: "go"},
				{Path: "README.md", LanguageID: "markdown"},
			},
			ActiveFile: &activeFile,
		},
		Terminal: &terminalstate.State{
			SchemaVersion: terminalstate.SchemaVersion,
			CapturedAt:    "2026-08-16T17:00:00Z",
			WorkingDirectories: []terminalstate.Directory{
				{Path: "."},
			},
		},
		Browser: &browserstate.State{
			SchemaVersion: browserstate.SchemaVersion,
			CapturedAt:    "2026-08-16T17:00:00Z",
			Tabs: []browserstate.Tab{
				{URL: "https://go.dev"},
				{URL: "https://pkg.go.dev"},
			},
		},
	}
}
