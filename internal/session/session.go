package session

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/gitstate"
)

const SchemaVersion = 1

type Source struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

type Session struct {
	SchemaVersion int            `json:"schema_version"`
	CapturedAt    time.Time      `json:"captured_at"`
	Source        Source         `json:"source"`
	Git           gitstate.State `json:"git"`
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
	return s, nil
}
