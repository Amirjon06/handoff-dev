package editorstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const SchemaVersion = 1

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Selection struct {
	Anchor Position `json:"anchor"`
	Active Position `json:"active"`
}

type File struct {
	Path       string      `json:"path"`
	LanguageID string      `json:"language_id"`
	IsDirty    bool        `json:"is_dirty"`
	Selections []Selection `json:"selections,omitempty"`
}

type State struct {
	SchemaVersion   int     `json:"schema_version"`
	CapturedAt      string  `json:"captured_at"`
	WorkspaceFolder *string `json:"workspace_folder"`
	ActiveFile      *string `json:"active_file"`
	OpenFiles       []File  `json:"open_files,omitempty"`
}

func ReadWorkspace(root string) (*State, error) {
	path := filepath.Join(root, ".staterelay", "editor-state.json")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	state, err := ReadJSON(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return &state, nil
}

func ReadJSON(r io.Reader) (State, error) {
	var state State
	if err := json.NewDecoder(r).Decode(&state); err != nil {
		return State{}, err
	}
	if err := Validate(&state); err != nil {
		return State{}, err
	}
	return state, nil
}

func Validate(state *State) error {
	if state == nil {
		return nil
	}
	if state.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported editor state schema version %d", state.SchemaVersion)
	}
	if strings.TrimSpace(state.CapturedAt) == "" {
		return fmt.Errorf("editor state captured_at is required")
	}
	for i, file := range state.OpenFiles {
		if strings.TrimSpace(file.Path) == "" {
			return fmt.Errorf("editor state open_files[%d].path is required", i)
		}
		if strings.TrimSpace(file.LanguageID) == "" {
			return fmt.Errorf("editor state open_files[%d].language_id is required", i)
		}
	}
	return nil
}
