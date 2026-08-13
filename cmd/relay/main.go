package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/session"
)

const version = "0.1.0-dev"

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "relay: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		printHelp(stdout)
		return nil
	}

	switch args[0] {
	case "capture":
		return runCapture(ctx, args[1:], stdout)
	case "restore":
		return runRestore(ctx, args[1:], stdout)
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	case "help", "-h", "--help":
		printHelp(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runCapture(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "project path to inspect")
	jsonOutput := fs.Bool("json", false, "write captured session as JSON")
	out := fs.String("out", "", "write captured session JSON to this file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	state, err := gitstate.Capture(ctx, *path)
	if err != nil {
		return err
	}

	if *jsonOutput || *out != "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		captured := session.New(hostname, state, time.Now())

		if *out == "" {
			return session.WriteJSON(stdout, captured)
		}

		file, err := os.Create(*out)
		if err != nil {
			return fmt.Errorf("create %s: %w", *out, err)
		}
		defer file.Close()

		if err := session.WriteJSON(file, captured); err != nil {
			return fmt.Errorf("write %s: %w", *out, err)
		}

		fmt.Fprintf(stdout, "Captured session to %s\n", *out)
		return nil
	}

	fmt.Fprintf(stdout, "Git root: %s\n", state.Root)
	fmt.Fprintf(stdout, "Git remote: %s\n", state.Remote)
	fmt.Fprintf(stdout, "Git branch: %s\n", state.Branch)
	fmt.Fprintf(stdout, "Git commit: %s\n", state.Commit)
	fmt.Fprintf(stdout, "Git dirty: %t\n", state.Dirty)
	for _, file := range state.ChangedFiles {
		fmt.Fprintf(stdout, "Changed file: %s %s\n", file.Status, file.Path)
	}
	return nil
}

func runRestore(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	apply := fs.Bool("apply", false, "write captured file snapshots after validation")
	dryRun := fs.Bool("dry-run", false, "validate apply without writing captured file snapshots")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("restore requires a session JSON file")
	}

	file, err := os.Open(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("open %s: %w", fs.Arg(0), err)
	}
	defer file.Close()

	captured, err := session.ReadJSON(file)
	if err != nil {
		return fmt.Errorf("read session: %w", err)
	}

	verifiedRoot, err := gitstate.Root(ctx, captured.Git.Root)
	if err != nil {
		return fmt.Errorf("verify git root: %w", err)
	}
	if !samePath(verifiedRoot, captured.Git.Root) {
		return fmt.Errorf("session root %s resolved to %s", captured.Git.Root, verifiedRoot)
	}

	currentBranch, err := gitstate.Branch(ctx, verifiedRoot)
	if err != nil {
		return fmt.Errorf("verify git branch: %w", err)
	}
	if currentBranch != captured.Git.Branch {
		return fmt.Errorf("session branch %s does not match current branch %s", captured.Git.Branch, currentBranch)
	}

	currentCommit, err := gitstate.Commit(ctx, verifiedRoot)
	if err != nil {
		return fmt.Errorf("verify git commit: %w", err)
	}
	if currentCommit != captured.Git.Commit {
		return fmt.Errorf("session commit %s does not match current commit %s", captured.Git.Commit, currentCommit)
	}

	if *apply {
		currentState, err := gitstate.Capture(ctx, verifiedRoot)
		if err != nil {
			return fmt.Errorf("verify working tree: %w", err)
		}
		if currentState.Dirty {
			return fmt.Errorf("refusing to apply over dirty working tree: %s", formatChangedFiles(currentState.ChangedFiles))
		}
		if *dryRun {
			result, err := planApplyFiles(verifiedRoot, captured.Git.ChangedFiles)
			if err != nil {
				return err
			}
			printApplyResult(stdout, "Would apply", result)
			return nil
		}
		result, err := applyChangedFiles(verifiedRoot, captured.Git.ChangedFiles)
		if err != nil {
			return err
		}
		printApplyResult(stdout, "Applied", result)
		return nil
	}

	fmt.Fprintln(stdout, "Restore plan")
	fmt.Fprintf(stdout, "Git root: %s\n", captured.Git.Root)
	fmt.Fprintf(stdout, "Git remote: %s\n", captured.Git.Remote)
	fmt.Fprintf(stdout, "Git branch: %s\n", captured.Git.Branch)
	fmt.Fprintf(stdout, "Git commit: %s\n", captured.Git.Commit)
	fmt.Fprintf(stdout, "Git dirty: %t\n", captured.Git.Dirty)
	if len(captured.Git.ChangedFiles) > 0 {
		fmt.Fprintln(stdout, "Changed files:")
		for _, file := range captured.Git.ChangedFiles {
			if file.ContentCaptured {
				fmt.Fprintf(stdout, "- %s %s (%d bytes captured)\n", file.Status, file.Path, file.Size)
				continue
			}
			fmt.Fprintf(stdout, "- %s %s (content not captured)\n", file.Status, file.Path)
		}
	}
	return nil
}

func formatChangedFiles(files []gitstate.ChangedFile) string {
	if len(files) == 0 {
		return "unknown local changes"
	}

	parts := make([]string, 0, len(files))
	for _, file := range files {
		parts = append(parts, file.Status+" "+file.Path)
	}
	return strings.Join(parts, ", ")
}

type applyResult struct {
	applied []gitstate.ChangedFile
	skipped []gitstate.ChangedFile
}

func planApplyFiles(root string, files []gitstate.ChangedFile) (applyResult, error) {
	var result applyResult

	for _, file := range files {
		if !file.ContentCaptured {
			result.skipped = append(result.skipped, file)
			continue
		}

		if _, ok := safePath(root, file.Path); !ok {
			return result, fmt.Errorf("unsafe changed file path %s", file.Path)
		}
		result.applied = append(result.applied, file)
	}

	return result, nil
}

func applyChangedFiles(root string, files []gitstate.ChangedFile) (applyResult, error) {
	result, err := planApplyFiles(root, files)
	if err != nil {
		return result, err
	}

	for _, file := range result.applied {
		path, _ := safePath(root, file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return result, fmt.Errorf("create parent directory for %s: %w", file.Path, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return result, fmt.Errorf("write changed file %s: %w", file.Path, err)
		}
	}

	return result, nil
}

func printApplyResult(stdout io.Writer, action string, result applyResult) {
	fmt.Fprintf(stdout, "%s %d changed file(s)\n", action, len(result.applied))
	if len(result.skipped) > 0 {
		fmt.Fprintf(stdout, "Skipped %d changed file(s) without captured content: %s\n", len(result.skipped), formatChangedFiles(result.skipped))
	}
}

func safePath(root string, path string) (string, bool) {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", false
	}
	return filepath.Join(root, cleanPath), true
}

func samePath(left string, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}

	return left == right
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "StateRelay")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  relay capture [--path PATH] [--json] [--out FILE]")
	fmt.Fprintln(w, "  relay restore [--apply] [--dry-run] SESSION_FILE")
	fmt.Fprintln(w, "  relay version")
}
