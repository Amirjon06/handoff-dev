package gitstate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxSnapshotBytes int64 = 1024 * 1024

type commandRunner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

type gitRunner struct{}

type ChangedFile struct {
	Path            string `json:"path"`
	Status          string `json:"status"`
	Size            int64  `json:"size,omitempty"`
	ContentCaptured bool   `json:"content_captured"`
	ContentEncoding string `json:"content_encoding,omitempty"`
	Content         string `json:"content,omitempty"`
}

type State struct {
	Root         string        `json:"root"`
	Name         string        `json:"name"`
	Remote       string        `json:"remote"`
	Branch       string        `json:"branch"`
	Commit       string        `json:"commit"`
	Dirty        bool          `json:"dirty"`
	ChangedFiles []ChangedFile `json:"changed_files,omitempty"`
}

func (gitRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}

	return strings.TrimRight(stdout.String(), "\r\n"), nil
}

func Root(ctx context.Context, path string) (string, error) {
	return root(ctx, gitRunner{}, path)
}

func Branch(ctx context.Context, path string) (string, error) {
	return branch(ctx, gitRunner{}, path)
}

func Commit(ctx context.Context, path string) (string, error) {
	return commit(ctx, gitRunner{}, path)
}

func Remote(ctx context.Context, path string) (string, error) {
	return remote(ctx, gitRunner{}, path)
}

func Capture(ctx context.Context, path string) (State, error) {
	return capture(ctx, gitRunner{}, path)
}

func capture(ctx context.Context, runner commandRunner, path string) (State, error) {
	root, err := root(ctx, runner, path)
	if err != nil {
		return State{}, err
	}

	remote, err := remote(ctx, runner, root)
	if err != nil {
		return State{}, err
	}

	branch, err := branch(ctx, runner, root)
	if err != nil {
		return State{}, err
	}

	commit, err := commit(ctx, runner, root)
	if err != nil {
		return State{}, err
	}

	changedFiles, err := changes(ctx, runner, root)
	if err != nil {
		return State{}, err
	}
	changedFiles, err = captureFileSnapshots(root, changedFiles)
	if err != nil {
		return State{}, err
	}

	return State{
		Root:         root,
		Name:         filepath.Base(root),
		Remote:       remote,
		Branch:       branch,
		Commit:       commit,
		Dirty:        len(changedFiles) > 0,
		ChangedFiles: changedFiles,
	}, nil
}

func root(ctx context.Context, runner commandRunner, path string) (string, error) {
	if path == "" {
		path = "."
	}

	gitRoot, err := runner.Run(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%s is not inside a git repository: %w", path, err)
	}

	absoluteRoot, err := filepath.Abs(gitRoot)
	if err != nil {
		return gitRoot, nil
	}

	return absoluteRoot, nil
}

func branch(ctx context.Context, runner commandRunner, path string) (string, error) {
	if path == "" {
		path = "."
	}

	currentBranch, err := runner.Run(ctx, path, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("read current git branch: %w", err)
	}
	if currentBranch == "" {
		return "", errors.New("repository is in detached HEAD state")
	}

	return currentBranch, nil
}

func commit(ctx context.Context, runner commandRunner, path string) (string, error) {
	if path == "" {
		path = "."
	}

	currentCommit, err := runner.Run(ctx, path, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("read current git commit: %w", err)
	}

	return currentCommit, nil
}

func remote(ctx context.Context, runner commandRunner, path string) (string, error) {
	if path == "" {
		path = "."
	}

	origin, err := runner.Run(ctx, path, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", fmt.Errorf("read git origin remote: %w", err)
	}
	if origin == "" {
		return "", errors.New("repository has no origin remote")
	}

	return origin, nil
}

func changes(ctx context.Context, runner commandRunner, path string) ([]ChangedFile, error) {
	if path == "" {
		path = "."
	}

	status, err := runner.Run(ctx, path, "status", "--porcelain=v1")
	if err != nil {
		return nil, fmt.Errorf("read git status: %w", err)
	}

	return parseStatus(status), nil
}

func parseStatus(status string) []ChangedFile {
	status = strings.TrimRight(status, "\r\n")
	if status == "" {
		return nil
	}

	lines := strings.Split(status, "\n")
	files := make([]ChangedFile, 0, len(lines))
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}

		code := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])
		if idx := strings.LastIndex(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}

		files = append(files, ChangedFile{
			Path:   path,
			Status: statusName(code),
		})
	}

	return files
}

func statusName(code string) string {
	switch {
	case code == "??":
		return "untracked"
	case strings.Contains(code, "A"):
		return "added"
	case strings.Contains(code, "D"):
		return "deleted"
	case strings.Contains(code, "R"):
		return "renamed"
	case strings.Contains(code, "C"):
		return "copied"
	case strings.Contains(code, "M"):
		return "modified"
	case strings.Contains(code, "U"):
		return "unmerged"
	default:
		return strings.ToLower(code)
	}
}

func captureFileSnapshots(root string, files []ChangedFile) ([]ChangedFile, error) {
	for i := range files {
		if files[i].Status == "deleted" {
			continue
		}

		fullPath, ok := safeJoin(root, files[i].Path)
		if !ok {
			continue
		}

		info, err := os.Stat(fullPath)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files[i].Size = info.Size()
		if info.Size() > maxSnapshotBytes {
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read changed file %s: %w", files[i].Path, err)
		}
		if !utf8.Valid(content) {
			continue
		}

		files[i].ContentCaptured = true
		files[i].ContentEncoding = "utf-8"
		files[i].Content = string(content)
	}

	return files, nil
}

func safeJoin(root string, path string) (string, bool) {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", false
	}

	return filepath.Join(root, cleanPath), true
}
