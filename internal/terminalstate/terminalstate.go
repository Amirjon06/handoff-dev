package terminalstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const SchemaVersion = 1

type Directory struct {
	Path string `json:"path"`
}

type State struct {
	SchemaVersion      int         `json:"schema_version"`
	CapturedAt         string      `json:"captured_at"`
	WorkingDirectories []Directory `json:"working_directories,omitempty"`
}

func Capture(root string, cwd string, capturedAt time.Time) (State, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return State{}, fmt.Errorf("read current directory: %w", err)
		}
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return State{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	rootAbs, err = normalizePath(rootAbs)
	if err != nil {
		return State{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return State{}, fmt.Errorf("resolve terminal directory: %w", err)
	}
	cwdAbs, err = normalizePath(cwdAbs)
	if err != nil {
		return State{}, fmt.Errorf("resolve terminal directory: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, cwdAbs)
	if err != nil {
		return State{}, fmt.Errorf("relativize terminal directory: %w", err)
	}
	if !safePath(rel) {
		return State{}, fmt.Errorf("terminal directory %s is outside workspace %s", cwdAbs, rootAbs)
	}
	if rel == "" {
		rel = "."
	}

	state := State{
		SchemaVersion: SchemaVersion,
		CapturedAt:    capturedAt.UTC().Format(time.RFC3339),
		WorkingDirectories: []Directory{
			{Path: filepath.ToSlash(rel)},
		},
	}
	if err := Validate(&state); err != nil {
		return State{}, err
	}
	return state, nil
}

func ReadWorkspace(root string) (*State, error) {
	path := filepath.Join(root, ".staterelay", "terminal-state.json")
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

func WriteWorkspace(root string, state *State) error {
	if state == nil {
		return nil
	}
	if err := Validate(state); err != nil {
		return err
	}

	dir := filepath.Join(root, ".staterelay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create terminal state directory: %w", err)
	}

	path := filepath.Join(dir, "terminal-state.json")
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()

	if err := WriteJSON(file, *state); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func ResolveDirectories(root string, state *State) ([]string, error) {
	if state == nil {
		return nil, nil
	}
	if err := Validate(state); err != nil {
		return nil, err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	rootAbs, err = normalizePath(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	dirs := make([]string, 0, len(state.WorkingDirectories))
	for _, dir := range state.WorkingDirectories {
		path := filepath.Join(rootAbs, filepath.FromSlash(dir.Path))
		normalized, err := normalizePath(path)
		if err != nil {
			return nil, fmt.Errorf("resolve terminal directory %s: %w", dir.Path, err)
		}
		rel, err := filepath.Rel(rootAbs, normalized)
		if err != nil {
			return nil, fmt.Errorf("relativize terminal directory %s: %w", dir.Path, err)
		}
		if !safePath(rel) {
			return nil, fmt.Errorf("terminal directory %s resolves outside workspace %s", dir.Path, rootAbs)
		}
		dirs = append(dirs, normalized)
	}
	return dirs, nil
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

func WriteJSON(w io.Writer, state State) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(state)
}

func Validate(state *State) error {
	if state == nil {
		return nil
	}
	if state.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported terminal state schema version %d", state.SchemaVersion)
	}
	if strings.TrimSpace(state.CapturedAt) == "" {
		return fmt.Errorf("terminal state captured_at is required")
	}
	for i, dir := range state.WorkingDirectories {
		if strings.TrimSpace(dir.Path) == "" {
			return fmt.Errorf("terminal state working_directories[%d].path is required", i)
		}
		if !safePath(dir.Path) {
			return fmt.Errorf("terminal state working_directories[%d].path %q is unsafe", i, dir.Path)
		}
	}
	return nil
}

func safePath(path string) bool {
	cleanPath := filepath.Clean(path)
	return !filepath.IsAbs(cleanPath) && cleanPath != ".." && !strings.HasPrefix(cleanPath, "../")
}

func normalizePath(path string) (string, error) {
	normalized, err := filepath.EvalSymlinks(path)
	if err == nil {
		return normalized, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return path, nil
	}
	return "", err
}
