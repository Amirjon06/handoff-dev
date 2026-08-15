package browserstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const SchemaVersion = 1

type Tab struct {
	URL string `json:"url"`
}

type State struct {
	SchemaVersion int    `json:"schema_version"`
	CapturedAt    string `json:"captured_at"`
	Tabs          []Tab  `json:"tabs,omitempty"`
}

func Capture(rawURLs []string, capturedAt time.Time) (State, error) {
	tabs := make([]Tab, 0, len(rawURLs))
	for _, rawURL := range rawURLs {
		cleanURL := strings.TrimSpace(rawURL)
		if cleanURL == "" {
			continue
		}
		if err := validateURL(cleanURL); err != nil {
			return State{}, err
		}
		tabs = append(tabs, Tab{URL: cleanURL})
	}
	if len(tabs) == 0 {
		return State{}, errors.New("at least one browser URL is required")
	}

	state := State{
		SchemaVersion: SchemaVersion,
		CapturedAt:    capturedAt.UTC().Format(time.RFC3339),
		Tabs:          tabs,
	}
	if err := Validate(&state); err != nil {
		return State{}, err
	}
	return state, nil
}

func ReadWorkspace(root string) (*State, error) {
	path := filepath.Join(root, ".staterelay", "browser-state.json")
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
		return fmt.Errorf("create browser state directory: %w", err)
	}

	path := filepath.Join(dir, "browser-state.json")
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
		return fmt.Errorf("unsupported browser state schema version %d", state.SchemaVersion)
	}
	if strings.TrimSpace(state.CapturedAt) == "" {
		return errors.New("browser state captured_at is required")
	}
	if len(state.Tabs) == 0 {
		return errors.New("browser state tabs are required")
	}
	for i, tab := range state.Tabs {
		if strings.TrimSpace(tab.URL) == "" {
			return fmt.Errorf("browser state tabs[%d].url is required", i)
		}
		if err := validateURL(tab.URL); err != nil {
			return fmt.Errorf("browser state tabs[%d].url: %w", i, err)
		}
	}
	return nil
}

func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse %q: %w", rawURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return errors.New("host is required")
	}
	return nil
}
