package transport

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/session"
)

func TestClientSendsSession(t *testing.T) {
	var received session.Session
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != SessionsPath {
				t.Fatalf("path = %q, want %q", r.URL.Path, SessionsPath)
			}
			if r.Method != http.MethodPost {
				t.Fatalf("method = %q, want POST", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("content type = %q", r.Header.Get("Content-Type"))
			}

			var err error
			received, err = session.ReadJSON(r.Body)
			if err != nil {
				t.Fatalf("ReadJSON returned error: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Status:     "202 Accepted",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"session-1","message":"session stored"}`)),
			}, nil
		}),
	}

	receipt, err := Client{HTTPClient: client}.Send(context.Background(), "http://127.0.0.1:8765", testSession())
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if receipt.ID != "session-1" {
		t.Fatalf("receipt id = %q", receipt.ID)
	}
	if received.Git.Name != "handoff-dev" {
		t.Fatalf("git name = %q", received.Git.Name)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClientRejectsTargetWithoutHTTP(t *testing.T) {
	_, err := Client{}.Send(context.Background(), "localhost:8765", testSession())
	if err == nil {
		t.Fatal("Send returned nil error for target without scheme")
	}
	if err.Error() != "target must start with http:// or https://" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestHandlerStoresValidSession(t *testing.T) {
	inbox := t.TempDir()
	var body bytes.Buffer
	if err := session.WriteJSON(&body, testSession()); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, SessionsPath, &body)
	rec := httptest.NewRecorder()
	Handler(FileInbox{Dir: inbox}).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"message":"session stored"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}

	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("stored file count = %d, want 1", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), ".json") {
		t.Fatalf("stored file = %q", entries[0].Name())
	}
}

func TestHandlerRejectsInvalidSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, SessionsPath, strings.NewReader(`{"schema_version":1}`))
	rec := httptest.NewRecorder()
	Handler(FileInbox{Dir: t.TempDir()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid session") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func testSession() session.Session {
	return session.New("test-machine", gitstate.State{
		Root:   "/repo/handoff-dev",
		Name:   "handoff-dev",
		Remote: "https://github.com/Amirjon06/handoff-dev.git",
		Branch: "main",
		Commit: "faaf307bf4fa86c316586804bf88f3096511aabd",
	}, time.Date(2026, 8, 15, 18, 30, 0, 0, time.UTC))
}
