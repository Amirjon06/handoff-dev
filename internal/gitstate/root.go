package gitstate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type commandRunner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

type gitRunner struct{}

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

	return strings.TrimSpace(stdout.String()), nil
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
