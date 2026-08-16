package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/session"
)

const (
	HealthPath   = "/health"
	SessionsPath = "/sessions"
)

var defaultHTTPClient = &http.Client{Timeout: 15 * time.Second}

type Receipt struct {
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

type Health struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type Client struct {
	HTTPClient *http.Client
}

func InsecureTLSClient() Client {
	return TLSClient(nil, true)
}

func TLSClient(certificates []tls.Certificate, insecureSkipVerify bool) Client {
	return Client{
		HTTPClient: &http.Client{
			Timeout: defaultHTTPClient.Timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates:       certificates,
					InsecureSkipVerify: insecureSkipVerify,
					MinVersion:         tls.VersionTLS12,
				},
			},
		},
	}
}

func (c Client) Ping(ctx context.Context, target string) (Health, error) {
	endpoint, err := targetEndpoint(target, HealthPath)
	if err != nil {
		return Health{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Health{}, err
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return Health{}, fmt.Errorf("ping listener: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Health{}, fmt.Errorf("ping listener: server returned %s", resp.Status)
	}

	var health Health
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return Health{}, fmt.Errorf("read health response: %w", err)
	}
	if health.Status != "ok" {
		return Health{}, fmt.Errorf("listener status %q", health.Status)
	}
	return health, nil
}

func (c Client) Send(ctx context.Context, target string, captured session.Session) (Receipt, error) {
	endpoint, err := targetEndpoint(target, SessionsPath)
	if err != nil {
		return Receipt{}, err
	}

	var body bytes.Buffer
	if err := session.WriteJSON(&body, captured); err != nil {
		return Receipt{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return Receipt{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client().Do(req)
	if err != nil {
		return Receipt{}, fmt.Errorf("send session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		detail := strings.TrimSpace(string(message))
		if detail == "" {
			return Receipt{}, fmt.Errorf("send session: server returned %s", resp.Status)
		}
		return Receipt{}, fmt.Errorf("send session: server returned %s: %s", resp.Status, detail)
	}

	var receipt Receipt
	if err := json.NewDecoder(resp.Body).Decode(&receipt); err != nil && !errors.Is(err, io.EOF) {
		return Receipt{}, fmt.Errorf("read send receipt: %w", err)
	}
	return receipt, nil
}

func Handler(inbox FileInbox) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(HealthPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Health{Status: "ok", Service: "staterelay"})
	})
	mux.HandleFunc(SessionsPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()

		captured, err := session.ReadJSON(io.LimitReader(r.Body, 10<<20))
		if err != nil {
			http.Error(w, "invalid session: "+err.Error(), http.StatusBadRequest)
			return
		}

		receipt, err := inbox.Save(r.Context(), captured)
		if err != nil {
			http.Error(w, "store session: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(receipt)
	})
	return mux
}

type FileInbox struct {
	Dir string
}

type InboxEntry struct {
	Path    string
	Name    string
	Session session.Session
}

func (i FileInbox) Save(ctx context.Context, captured session.Session) (Receipt, error) {
	select {
	case <-ctx.Done():
		return Receipt{}, ctx.Err()
	default:
	}

	dir := i.dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Receipt{}, fmt.Errorf("create inbox: %w", err)
	}

	base := sessionFileBase(captured)
	for attempt := 0; attempt < 1000; attempt++ {
		id := base
		if attempt > 0 {
			id += "-" + strconv.Itoa(attempt+1)
		}
		path := filepath.Join(dir, id+".json")
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return Receipt{}, fmt.Errorf("create %s: %w", path, err)
		}

		writeErr := session.WriteJSON(file, captured)
		closeErr := file.Close()
		if writeErr != nil {
			return Receipt{}, fmt.Errorf("write %s: %w", path, writeErr)
		}
		if closeErr != nil {
			return Receipt{}, fmt.Errorf("close %s: %w", path, closeErr)
		}
		return Receipt{ID: id, Message: "session stored"}, nil
	}

	return Receipt{}, fmt.Errorf("could not allocate session file name")
}

func (i FileInbox) List(ctx context.Context) ([]InboxEntry, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	dir := i.dir()
	files, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read inbox: %w", err)
	}

	entries := make([]InboxEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, file.Name())
		handle, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		captured, readErr := session.ReadJSON(handle)
		closeErr := handle.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", path, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", path, closeErr)
		}

		entries = append(entries, InboxEntry{
			Path:    path,
			Name:    file.Name(),
			Session: captured,
		})
	}

	sort.Slice(entries, func(i int, j int) bool {
		left := entries[i].Session.CapturedAt
		right := entries[j].Session.CapturedAt
		if left.Equal(right) {
			return entries[i].Name < entries[j].Name
		}
		return left.After(right)
	})

	return entries, nil
}

func (i FileInbox) dir() string {
	if strings.TrimSpace(i.Dir) == "" {
		return filepath.Join(".staterelay", "inbox")
	}
	return i.Dir
}

func (c Client) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return defaultHTTPClient
}

func targetEndpoint(target string, defaultPath string) (string, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("parse target: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("target must start with http:// or https://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("target host is required")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = defaultPath
	}
	return parsed.String(), nil
}

func sessionFileBase(captured session.Session) string {
	commit := captured.Git.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	return captured.CapturedAt.UTC().Format("20060102T150405Z") + "-" + slug(captured.Git.Name) + "-" + slug(commit)
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if ok {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}
