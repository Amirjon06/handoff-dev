package editorstate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWorkspaceReturnsNilWhenMissing(t *testing.T) {
	state, err := ReadWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("ReadWorkspace returned error: %v", err)
	}
	if state != nil {
		t.Fatalf("state = %#v, want nil", state)
	}
}

func TestReadWorkspaceReadsEditorState(t *testing.T) {
	root := t.TempDir()
	writeEditorState(t, root, `{
		"schema_version": 1,
		"captured_at": "2026-08-15T18:30:00Z",
		"workspace_folder": "/repo",
		"active_file": "README.md",
		"open_files": [
			{
				"path": "README.md",
				"language_id": "markdown",
				"is_dirty": true,
				"selections": [
					{
						"anchor": { "line": 2, "character": 4 },
						"active": { "line": 2, "character": 12 }
					}
				]
			}
		]
	}`)

	state, err := ReadWorkspace(root)
	if err != nil {
		t.Fatalf("ReadWorkspace returned error: %v", err)
	}
	if state == nil {
		t.Fatal("state = nil, want editor state")
	}
	if state.ActiveFile == nil || *state.ActiveFile != "README.md" {
		t.Fatalf("active file = %#v", state.ActiveFile)
	}
	if len(state.OpenFiles) != 1 {
		t.Fatalf("open file count = %d, want 1", len(state.OpenFiles))
	}
	if state.OpenFiles[0].Selections[0].Active.Character != 12 {
		t.Fatalf("active character = %d, want 12", state.OpenFiles[0].Selections[0].Active.Character)
	}
}

func TestReadJSONRejectsMissingOpenFilePath(t *testing.T) {
	root := t.TempDir()
	writeEditorState(t, root, `{
		"schema_version": 1,
		"captured_at": "2026-08-15T18:30:00Z",
		"open_files": [
			{
				"language_id": "go",
				"is_dirty": false
			}
		]
	}`)

	_, err := ReadWorkspace(root)
	if err == nil {
		t.Fatal("ReadWorkspace returned nil error without open file path")
	}
	if got := err.Error(); got != "read "+filepath.Join(root, ".staterelay", "editor-state.json")+": editor state open_files[0].path is required" {
		t.Fatalf("error = %q", got)
	}
}

func writeEditorState(t *testing.T, root string, content string) {
	t.Helper()

	dir := filepath.Join(root, ".staterelay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "editor-state.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
