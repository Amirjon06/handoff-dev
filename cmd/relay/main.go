package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/editorstate"
	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/session"
	"github.com/Amirjon06/handoff-dev/internal/transport"
)

const version = "0.1.0-dev"

var sendSession = transport.Client{}.Send
var pingListener = transport.Client{}.Ping

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
	case "send":
		return runSend(ctx, args[1:], stdout)
	case "ping":
		return runPing(ctx, args[1:], stdout)
	case "listen":
		return runListen(ctx, args[1:], stdout)
	case "inbox":
		return runInbox(ctx, args[1:], stdout)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout)
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

func runDoctor(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "project path to inspect")
	inbox := fs.String("inbox", filepath.Join(".staterelay", "inbox"), "directory for received sessions")
	target := fs.String("to", "", "optional HTTP listener to check")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("doctor does not accept positional arguments")
	}

	state, err := gitstate.Capture(ctx, *path)
	if err != nil {
		return fmt.Errorf("check repository: %w", err)
	}
	entries, err := transport.FileInbox{Dir: *inbox}.List(ctx)
	if err != nil {
		return fmt.Errorf("check inbox: %w", err)
	}

	fmt.Fprintln(stdout, "StateRelay doctor")
	fmt.Fprintln(stdout, "Repository: ok")
	fmt.Fprintf(stdout, "  root: %s\n", state.Root)
	fmt.Fprintf(stdout, "  branch: %s\n", state.Branch)
	fmt.Fprintf(stdout, "  commit: %s\n", shortHash(state.Commit))
	fmt.Fprintf(stdout, "  dirty: %t\n", state.Dirty)
	fmt.Fprintln(stdout, "Inbox: ok")
	fmt.Fprintf(stdout, "  path: %s\n", *inbox)
	fmt.Fprintf(stdout, "  sessions: %d\n", len(entries))

	if *target == "" {
		fmt.Fprintln(stdout, "Listener: skipped")
		return nil
	}
	health, err := pingListener(ctx, *target)
	if err != nil {
		return fmt.Errorf("check listener: %w", err)
	}
	fmt.Fprintln(stdout, "Listener: ok")
	fmt.Fprintf(stdout, "  target: %s\n", *target)
	fmt.Fprintf(stdout, "  service: %s\n", health.Service)
	return nil
}

func runPing(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	target := fs.String("to", "", "HTTP listener to check")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("ping requires --to")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("ping does not accept positional arguments")
	}

	health, err := pingListener(ctx, *target)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Listener: %s\n", *target)
	fmt.Fprintf(stdout, "Status: %s\n", health.Status)
	fmt.Fprintf(stdout, "Service: %s\n", health.Service)
	return nil
}

func runSend(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	target := fs.String("to", "", "HTTP target to send the session to")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("send requires --to")
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("send requires a session JSON file")
	}

	captured, err := readSessionFile(fs.Arg(0))
	if err != nil {
		return err
	}

	receipt, err := sendSession(ctx, *target, captured)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Sent session to %s\n", *target)
	if receipt.ID != "" {
		fmt.Fprintf(stdout, "Receipt: %s\n", receipt.ID)
	}
	return nil
}

func runListen(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("listen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	addr := fs.String("addr", "127.0.0.1:8765", "HTTP listen address")
	inbox := fs.String("inbox", filepath.Join(".staterelay", "inbox"), "directory for received sessions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("listen does not accept positional arguments")
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           transport.Handler(transport.FileInbox{Dir: *inbox}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(stdout, "Listening on http://%s\n", *addr)
	fmt.Fprintf(stdout, "Inbox: %s\n", *inbox)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runInbox(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("inbox", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	inbox := fs.String("inbox", filepath.Join(".staterelay", "inbox"), "directory for received sessions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("inbox does not accept positional arguments")
	}

	entries, err := transport.FileInbox{Dir: *inbox}.List(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Inbox: %s\n", *inbox)
	if len(entries) == 0 {
		fmt.Fprintln(stdout, "No received sessions")
		return nil
	}
	for _, entry := range entries {
		captured := entry.Session
		fmt.Fprintf(stdout, "%s\n", entry.Path)
		fmt.Fprintf(stdout, "  repo: %s\n", captured.Git.Name)
		fmt.Fprintf(stdout, "  branch: %s\n", captured.Git.Branch)
		fmt.Fprintf(stdout, "  commit: %s\n", shortHash(captured.Git.Commit))
		fmt.Fprintf(stdout, "  dirty: %t\n", captured.Git.Dirty)
		fmt.Fprintf(stdout, "  changed files: %d\n", len(captured.Git.ChangedFiles))
		if captured.Editor != nil {
			fmt.Fprintf(stdout, "  editor files: %d\n", len(captured.Editor.OpenFiles))
		}
	}
	return nil
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
	editor, err := editorstate.ReadWorkspace(state.Root)
	if err != nil {
		return fmt.Errorf("read editor state: %w", err)
	}

	if *jsonOutput || *out != "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		captured := session.NewWithEditor(hostname, state, editor, time.Now())

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
	fmt.Fprintf(stdout, "Git name: %s\n", state.Name)
	fmt.Fprintf(stdout, "Git remote: %s\n", state.Remote)
	fmt.Fprintf(stdout, "Git branch: %s\n", state.Branch)
	fmt.Fprintf(stdout, "Git commit: %s\n", state.Commit)
	fmt.Fprintf(stdout, "Git dirty: %t\n", state.Dirty)
	for _, file := range state.ChangedFiles {
		if file.ContentCaptured {
			fmt.Fprintf(stdout, "Changed file: %s %s (%d bytes captured, sha256 %s)\n", file.Status, file.Path, file.Size, shortHash(file.ContentSHA256))
			continue
		}
		fmt.Fprintf(stdout, "Changed file: %s %s (content not captured)\n", file.Status, file.Path)
	}
	printEditorState(stdout, editor)
	return nil
}

func runRestore(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	apply := fs.Bool("apply", false, "write captured file snapshots after validation")
	dryRun := fs.Bool("dry-run", false, "validate apply without writing captured file snapshots")
	path := fs.String("path", "", "project path to restore into")
	cloneDir := fs.String("clone-dir", "", "clone missing repository into this parent directory")
	inbox := fs.String("inbox", "", "directory for received sessions; use latest as the session name")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("restore requires a session JSON file")
	}

	sessionPath, err := resolveRestoreSession(ctx, fs.Arg(0), *inbox)
	if err != nil {
		return err
	}

	captured, err := readSessionFile(sessionPath)
	if err != nil {
		return err
	}

	restorePath, cloned, err := resolveRestorePath(ctx, captured.Git, *path, *cloneDir)
	if err != nil {
		return err
	}

	verifiedRoot, err := gitstate.Root(ctx, restorePath)
	if err != nil {
		return fmt.Errorf("verify git root: %w", err)
	}
	if *path == "" && !samePath(verifiedRoot, captured.Git.Root) {
		if *cloneDir == "" {
			return fmt.Errorf("session root %s resolved to %s", captured.Git.Root, verifiedRoot)
		}
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
			printEditorApplyResult(stdout, "Would restore", captured.Editor)
			return nil
		}
		result, err := applyChangedFiles(verifiedRoot, captured.Git.ChangedFiles)
		if err != nil {
			return err
		}
		if err := editorstate.WriteWorkspace(verifiedRoot, captured.Editor); err != nil {
			return err
		}
		printApplyResult(stdout, "Applied", result)
		printEditorApplyResult(stdout, "Restored", captured.Editor)
		return nil
	}

	fmt.Fprintln(stdout, "Restore plan")
	fmt.Fprintf(stdout, "Session file: %s\n", sessionPath)
	if cloned {
		fmt.Fprintf(stdout, "Cloned repository to: %s\n", verifiedRoot)
	}
	fmt.Fprintf(stdout, "Git root: %s\n", captured.Git.Root)
	if *path != "" {
		fmt.Fprintf(stdout, "Restore root: %s\n", verifiedRoot)
	}
	if *cloneDir != "" {
		fmt.Fprintf(stdout, "Restore root: %s\n", verifiedRoot)
	}
	fmt.Fprintf(stdout, "Git name: %s\n", captured.Git.Name)
	fmt.Fprintf(stdout, "Git remote: %s\n", captured.Git.Remote)
	fmt.Fprintf(stdout, "Git branch: %s\n", captured.Git.Branch)
	fmt.Fprintf(stdout, "Git commit: %s\n", captured.Git.Commit)
	fmt.Fprintf(stdout, "Git dirty: %t\n", captured.Git.Dirty)
	if len(captured.Git.ChangedFiles) > 0 {
		fmt.Fprintln(stdout, "Changed files:")
		for _, file := range captured.Git.ChangedFiles {
			if file.ContentCaptured {
				fmt.Fprintf(stdout, "- %s %s (%d bytes captured, sha256 %s)\n", file.Status, file.Path, file.Size, shortHash(file.ContentSHA256))
				continue
			}
			fmt.Fprintf(stdout, "- %s %s (content not captured)\n", file.Status, file.Path)
		}
	}
	printEditorState(stdout, captured.Editor)
	return nil
}

func resolveRestorePath(ctx context.Context, git gitstate.State, path string, cloneDir string) (string, bool, error) {
	if path != "" && cloneDir != "" {
		return "", false, fmt.Errorf("restore cannot use --path and --clone-dir together")
	}
	if path != "" {
		return path, false, nil
	}
	if cloneDir == "" {
		return git.Root, false, nil
	}
	if !safeRepoName(git.Name) {
		return "", false, fmt.Errorf("unsafe repository name %q", git.Name)
	}

	destination := filepath.Join(cloneDir, git.Name)
	if _, err := os.Stat(destination); err == nil {
		return destination, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("inspect clone destination: %w", err)
	}

	if err := gitstate.Clone(ctx, git.Remote, git.Branch, destination); err != nil {
		return "", false, err
	}
	return destination, true, nil
}

func safeRepoName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return false
	}
	return filepath.Clean(name) == name
}

func resolveRestoreSession(ctx context.Context, name string, inbox string) (string, error) {
	if inbox == "" && name != "latest" {
		return name, nil
	}

	inboxDir := inbox
	if inboxDir == "" {
		inboxDir = filepath.Join(".staterelay", "inbox")
	}

	entries, err := transport.FileInbox{Dir: inboxDir}.List(ctx)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no received sessions in %s", inboxDir)
	}
	if name == "latest" {
		return entries[0].Path, nil
	}
	for _, entry := range entries {
		if entry.Name == name || strings.TrimSuffix(entry.Name, ".json") == name {
			return entry.Path, nil
		}
	}
	return "", fmt.Errorf("received session %s not found in %s", name, inboxDir)
}

func readSessionFile(path string) (session.Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return session.Session{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	captured, err := session.ReadJSON(file)
	if err != nil {
		return session.Session{}, fmt.Errorf("read session: %w", err)
	}
	return captured, nil
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
		if file.ContentSHA256 == "" {
			return result, fmt.Errorf("changed file %s is missing content hash", file.Path)
		}
		if sha256Hex([]byte(file.Content)) != file.ContentSHA256 {
			return result, fmt.Errorf("changed file %s content hash mismatch", file.Path)
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

func printEditorState(stdout io.Writer, editor *editorstate.State) {
	if editor == nil {
		return
	}
	fmt.Fprintf(stdout, "Editor state: %d open file(s)\n", len(editor.OpenFiles))
	if editor.ActiveFile != nil {
		fmt.Fprintf(stdout, "Active editor file: %s\n", *editor.ActiveFile)
	}
}

func printEditorApplyResult(stdout io.Writer, action string, editor *editorstate.State) {
	if editor == nil {
		return
	}
	fmt.Fprintf(stdout, "%s editor state: %d open file(s)\n", action, len(editor.OpenFiles))
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
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
	fmt.Fprintln(w, "  relay restore [--path PATH] [--clone-dir DIR] [--inbox DIR] [--apply] [--dry-run] SESSION_FILE|latest")
	fmt.Fprintln(w, "  relay listen [--addr ADDR] [--inbox DIR]")
	fmt.Fprintln(w, "  relay inbox [--inbox DIR]")
	fmt.Fprintln(w, "  relay doctor [--path PATH] [--inbox DIR] [--to URL]")
	fmt.Fprintln(w, "  relay ping --to URL")
	fmt.Fprintln(w, "  relay send --to URL SESSION_FILE")
	fmt.Fprintln(w, "  relay version")
}
