package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
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

	if err := fs.Parse(args); err != nil {
		return err
	}

	state, err := gitstate.Capture(ctx, *path)
	if err != nil {
		return err
	}

	if *jsonOutput {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		return session.WriteJSON(stdout, session.New(hostname, state, time.Now()))
	}

	fmt.Fprintf(stdout, "Git root: %s\n", state.Root)
	fmt.Fprintf(stdout, "Git remote: %s\n", state.Remote)
	fmt.Fprintf(stdout, "Git branch: %s\n", state.Branch)
	fmt.Fprintf(stdout, "Git commit: %s\n", state.Commit)
	return nil
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "StateRelay")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  relay capture [--path PATH] [--json]")
	fmt.Fprintln(w, "  relay version")
}
