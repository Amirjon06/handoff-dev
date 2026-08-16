package history

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/session"
	_ "modernc.org/sqlite"
)

const DirectionReceived = "received"

type Store struct {
	db *sql.DB
}

type Event struct {
	ID                string
	Direction         string
	SessionPath       string
	StoredAt          time.Time
	CapturedAt        time.Time
	SourceHostname    string
	RepoName          string
	RepoRemote        string
	RepoBranch        string
	RepoCommit        string
	Dirty             bool
	ChangedFiles      int
	EditorFiles       int
	TerminalDirs      int
	BrowserTabs       int
	SignerName        string
	SignerFingerprint string
}

func Path(root string) string {
	return filepath.Join(root, ".staterelay", "history.db")
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		path = Path(".")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func NewReceivedEvent(id string, sessionPath string, captured session.Session, storedAt time.Time) Event {
	event := Event{
		ID:             strings.TrimSpace(id),
		Direction:      DirectionReceived,
		SessionPath:    sessionPath,
		StoredAt:       storedAt.UTC(),
		CapturedAt:     captured.CapturedAt.UTC(),
		SourceHostname: captured.Source.Hostname,
		RepoName:       captured.Git.Name,
		RepoRemote:     captured.Git.Remote,
		RepoBranch:     captured.Git.Branch,
		RepoCommit:     captured.Git.Commit,
		Dirty:          captured.Git.Dirty,
		ChangedFiles:   len(captured.Git.ChangedFiles),
	}
	if captured.Editor != nil {
		event.EditorFiles = len(captured.Editor.OpenFiles)
	}
	if captured.Terminal != nil {
		event.TerminalDirs = len(captured.Terminal.WorkingDirectories)
	}
	if captured.Browser != nil {
		event.BrowserTabs = len(captured.Browser.Tabs)
	}
	if captured.Signature != nil {
		event.SignerName = captured.Signature.DeviceName
		event.SignerFingerprint = captured.Signature.Fingerprint
	}
	return event
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Record(ctx context.Context, event Event) error {
	if strings.TrimSpace(event.ID) == "" {
		return fmt.Errorf("history id is required")
	}
	if event.Direction != DirectionReceived {
		return fmt.Errorf("unsupported history direction %q", event.Direction)
	}
	if event.StoredAt.IsZero() {
		return fmt.Errorf("history stored_at is required")
	}
	if event.CapturedAt.IsZero() {
		return fmt.Errorf("history captured_at is required")
	}

	_, err := s.db.ExecContext(ctx, `
INSERT OR REPLACE INTO session_history (
	id,
	direction,
	session_path,
	stored_at,
	captured_at,
	source_hostname,
	repo_name,
	repo_remote,
	repo_branch,
	repo_commit,
	dirty,
	changed_files,
	editor_files,
	terminal_dirs,
	browser_tabs,
	signer_name,
	signer_fingerprint
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		event.Direction,
		event.SessionPath,
		formatTime(event.StoredAt),
		formatTime(event.CapturedAt),
		event.SourceHostname,
		event.RepoName,
		event.RepoRemote,
		event.RepoBranch,
		event.RepoCommit,
		boolInt(event.Dirty),
		event.ChangedFiles,
		event.EditorFiles,
		event.TerminalDirs,
		event.BrowserTabs,
		event.SignerName,
		event.SignerFingerprint,
	)
	if err != nil {
		return fmt.Errorf("insert session history: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT
	id,
	direction,
	session_path,
	stored_at,
	captured_at,
	source_hostname,
	repo_name,
	repo_remote,
	repo_branch,
	repo_commit,
	dirty,
	changed_files,
	editor_files,
	terminal_dirs,
	browser_tabs,
	signer_name,
	signer_fingerprint
FROM session_history
ORDER BY stored_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list session history: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var storedAt string
		var capturedAt string
		var dirty int
		if err := rows.Scan(
			&event.ID,
			&event.Direction,
			&event.SessionPath,
			&storedAt,
			&capturedAt,
			&event.SourceHostname,
			&event.RepoName,
			&event.RepoRemote,
			&event.RepoBranch,
			&event.RepoCommit,
			&dirty,
			&event.ChangedFiles,
			&event.EditorFiles,
			&event.TerminalDirs,
			&event.BrowserTabs,
			&event.SignerName,
			&event.SignerFingerprint,
		); err != nil {
			return nil, fmt.Errorf("scan session history: %w", err)
		}
		parsedStoredAt, err := parseTime(storedAt)
		if err != nil {
			return nil, fmt.Errorf("parse stored_at for %s: %w", event.ID, err)
		}
		parsedCapturedAt, err := parseTime(capturedAt)
		if err != nil {
			return nil, fmt.Errorf("parse captured_at for %s: %w", event.ID, err)
		}
		event.StoredAt = parsedStoredAt
		event.CapturedAt = parsedCapturedAt
		event.Dirty = dirty != 0
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read session history: %w", err)
	}
	return events, nil
}

func (s *Store) Count(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_history`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count session history: %w", err)
	}
	return count, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS session_history (
	id TEXT PRIMARY KEY,
	direction TEXT NOT NULL,
	session_path TEXT NOT NULL,
	stored_at TEXT NOT NULL,
	captured_at TEXT NOT NULL,
	source_hostname TEXT NOT NULL,
	repo_name TEXT NOT NULL,
	repo_remote TEXT NOT NULL,
	repo_branch TEXT NOT NULL,
	repo_commit TEXT NOT NULL,
	dirty INTEGER NOT NULL,
	changed_files INTEGER NOT NULL,
	editor_files INTEGER NOT NULL,
	terminal_dirs INTEGER NOT NULL,
	browser_tabs INTEGER NOT NULL,
	signer_name TEXT NOT NULL DEFAULT '',
	signer_fingerprint TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_session_history_stored_at ON session_history(stored_at);
CREATE INDEX IF NOT EXISTS idx_session_history_repo ON session_history(repo_name, repo_branch);`)
	if err != nil {
		return fmt.Errorf("migrate history database: %w", err)
	}
	return nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
