package session

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/editorstate"
	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/terminalstate"
)

const SchemaVersion = 1

type Source struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

type Session struct {
	SchemaVersion int                  `json:"schema_version"`
	CapturedAt    time.Time            `json:"captured_at"`
	Source        Source               `json:"source"`
	Git           gitstate.State       `json:"git"`
	Editor        *editorstate.State   `json:"editor,omitempty"`
	Terminal      *terminalstate.State `json:"terminal,omitempty"`
}

func New(hostname string, git gitstate.State, capturedAt time.Time) Session {
	return Session{
		SchemaVersion: SchemaVersion,
		CapturedAt:    capturedAt.UTC(),
		Source: Source{
			Hostname: hostname,
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
		},
		Git: git,
	}
}

func NewWithEditor(hostname string, git gitstate.State, editor *editorstate.State, capturedAt time.Time) Session {
	s := New(hostname, git, capturedAt)
	s.Editor = editor
	return s
}

func NewWithWorkspace(hostname string, git gitstate.State, editor *editorstate.State, terminal *terminalstate.State, capturedAt time.Time) Session {
	s := NewWithEditor(hostname, git, editor, capturedAt)
	s.Terminal = terminal
	return s
}

func WriteJSON(w io.Writer, s Session) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

func ReadJSON(r io.Reader) (Session, error) {
	var s Session
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return Session{}, err
	}
	if s.SchemaVersion != SchemaVersion {
		return Session{}, fmt.Errorf("unsupported session schema version %d", s.SchemaVersion)
	}
	if err := validate(s); err != nil {
		return Session{}, err
	}
	return s, nil
}

func validate(s Session) error {
	required := []struct {
		name  string
		value string
	}{
		{"source.hostname", s.Source.Hostname},
		{"source.os", s.Source.OS},
		{"source.arch", s.Source.Arch},
		{"git.root", s.Git.Root},
		{"git.name", s.Git.Name},
		{"git.remote", s.Git.Remote},
		{"git.branch", s.Git.Branch},
		{"git.commit", s.Git.Commit},
	}

	if s.CapturedAt.IsZero() {
		return fmt.Errorf("session captured_at is required")
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("session %s is required", field.name)
		}
	}
	if err := validateChangedFiles(s.Git.ChangedFiles); err != nil {
		return err
	}
	if err := editorstate.Validate(s.Editor); err != nil {
		return fmt.Errorf("session editor: %w", err)
	}
	if err := terminalstate.Validate(s.Terminal); err != nil {
		return fmt.Errorf("session terminal: %w", err)
	}

	return nil
}

func validateChangedFiles(files []gitstate.ChangedFile) error {
	for i, file := range files {
		prefix := fmt.Sprintf("git.changed_files[%d]", i)
		if strings.TrimSpace(file.Path) == "" {
			return fmt.Errorf("session %s.path is required", prefix)
		}
		if !safeSessionPath(file.Path) {
			return fmt.Errorf("session %s.path %q is unsafe", prefix, file.Path)
		}
		if strings.TrimSpace(file.Status) == "" {
			return fmt.Errorf("session %s.status is required", prefix)
		}
		if !file.ContentCaptured {
			continue
		}
		if strings.TrimSpace(file.ContentEncoding) == "" {
			return fmt.Errorf("session %s.content_encoding is required", prefix)
		}
		if file.ContentEncoding != "utf-8" {
			return fmt.Errorf("session %s.content_encoding %q is unsupported", prefix, file.ContentEncoding)
		}
		if strings.TrimSpace(file.ContentSHA256) == "" {
			return fmt.Errorf("session %s.content_sha256 is required", prefix)
		}
	}

	return nil
}

func safeSessionPath(path string) bool {
	cleanPath := filepath.Clean(path)
	return !filepath.IsAbs(cleanPath) && cleanPath != ".." && !strings.HasPrefix(cleanPath, "../")
}
