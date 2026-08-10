package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"version"}, &stdout)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != version {
		t.Fatalf("version output = %q, want %q", got, version)
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer

	err := run(context.Background(), []string{"nope"}, &stdout)
	if err == nil {
		t.Fatal("run returned nil error for unknown command")
	}
}
