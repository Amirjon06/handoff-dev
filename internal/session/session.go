package session

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/browserstate"
	"github.com/Amirjon06/handoff-dev/internal/deviceidentity"
	"github.com/Amirjon06/handoff-dev/internal/editorstate"
	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/terminalstate"
)

const SchemaVersion = 1
const SignatureAlgorithm = "ed25519"

type Source struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

type Session struct {
	SchemaVersion int                  `json:"schema_version"`
	CapturedAt    time.Time            `json:"captured_at"`
	Source        Source               `json:"source"`
	Git           gitstate.State       `json:"git"`
	Editor        *editorstate.State   `json:"editor,omitempty"`
	Terminal      *terminalstate.State `json:"terminal,omitempty"`
	Browser       *browserstate.State  `json:"browser,omitempty"`
	Signature     *Signature           `json:"signature,omitempty"`
}

type Signature struct {
	Algorithm   string `json:"algorithm"`
	DeviceName  string `json:"device_name"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	Value       string `json:"value"`
}

func New(hostname string, git gitstate.State, capturedAt time.Time) Session {
	return Session{
		SchemaVersion: SchemaVersion,
		CapturedAt:    capturedAt.UTC(),
		Source: Source{
			Hostname: hostname,
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
		},
		Git: git,
	}
}

func NewWithEditor(hostname string, git gitstate.State, editor *editorstate.State, capturedAt time.Time) Session {
	s := New(hostname, git, capturedAt)
	s.Editor = editor
	return s
}

func NewWithWorkspace(hostname string, git gitstate.State, editor *editorstate.State, terminal *terminalstate.State, browser *browserstate.State, capturedAt time.Time) Session {
	s := NewWithEditor(hostname, git, editor, capturedAt)
	s.Terminal = terminal
	s.Browser = browser
	return s
}

func WriteJSON(w io.Writer, s Session) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

func Sign(s Session, identity deviceidentity.Identity) (Session, error) {
	if err := deviceidentity.Validate(identity); err != nil {
		return Session{}, err
	}
	seed, err := base64.StdEncoding.DecodeString(identity.PrivateKeySeed)
	if err != nil {
		return Session{}, fmt.Errorf("decode private key seed: %w", err)
	}
	payload, err := signingPayload(s)
	if err != nil {
		return Session{}, err
	}
	signature := ed25519.Sign(ed25519.NewKeyFromSeed(seed), payload)
	s.Signature = &Signature{
		Algorithm:   SignatureAlgorithm,
		DeviceName:  identity.Name,
		PublicKey:   identity.PublicKey,
		Fingerprint: identity.Fingerprint,
		Value:       base64.StdEncoding.EncodeToString(signature),
	}
	return s, nil
}

func VerifySignature(s Session) error {
	if s.Signature == nil {
		return fmt.Errorf("session signature is missing")
	}
	if err := validateSignatureFields(*s.Signature); err != nil {
		return err
	}
	publicKey, err := base64.StdEncoding.DecodeString(s.Signature.PublicKey)
	if err != nil {
		return fmt.Errorf("decode signature public key: %w", err)
	}
	signature, err := base64.StdEncoding.DecodeString(s.Signature.Value)
	if err != nil {
		return fmt.Errorf("decode signature value: %w", err)
	}
	payload, err := signingPayload(s)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("session signature verification failed")
	}
	return nil
}

func ReadJSON(r io.Reader) (Session, error) {
	var s Session
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return Session{}, err
	}
	if s.SchemaVersion != SchemaVersion {
		return Session{}, fmt.Errorf("unsupported session schema version %d", s.SchemaVersion)
	}
	if err := validate(s); err != nil {
		return Session{}, err
	}
	return s, nil
}

func validate(s Session) error {
	required := []struct {
		name  string
		value string
	}{
		{"source.hostname", s.Source.Hostname},
		{"source.os", s.Source.OS},
		{"source.arch", s.Source.Arch},
		{"git.root", s.Git.Root},
		{"git.name", s.Git.Name},
		{"git.remote", s.Git.Remote},
		{"git.branch", s.Git.Branch},
		{"git.commit", s.Git.Commit},
	}

	if s.CapturedAt.IsZero() {
		return fmt.Errorf("session captured_at is required")
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("session %s is required", field.name)
		}
	}
	if err := validateChangedFiles(s.Git.ChangedFiles); err != nil {
		return err
	}
	if err := editorstate.Validate(s.Editor); err != nil {
		return fmt.Errorf("session editor: %w", err)
	}
	if err := terminalstate.Validate(s.Terminal); err != nil {
		return fmt.Errorf("session terminal: %w", err)
	}
	if err := browserstate.Validate(s.Browser); err != nil {
		return fmt.Errorf("session browser: %w", err)
	}
	if s.Signature != nil {
		if err := VerifySignature(s); err != nil {
			return fmt.Errorf("session signature: %w", err)
		}
	}

	return nil
}

func validateSignatureFields(signature Signature) error {
	if signature.Algorithm != SignatureAlgorithm {
		return fmt.Errorf("unsupported algorithm %q", signature.Algorithm)
	}
	if strings.TrimSpace(signature.DeviceName) == "" {
		return fmt.Errorf("device_name is required")
	}
	publicKey, err := base64.StdEncoding.DecodeString(signature.PublicKey)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("public key has %d bytes, want %d", len(publicKey), ed25519.PublicKeySize)
	}
	if signature.Fingerprint != fingerprint(publicKey) {
		return fmt.Errorf("fingerprint does not match public key")
	}
	value, err := base64.StdEncoding.DecodeString(signature.Value)
	if err != nil {
		return fmt.Errorf("decode value: %w", err)
	}
	if len(value) != ed25519.SignatureSize {
		return fmt.Errorf("value has %d bytes, want %d", len(value), ed25519.SignatureSize)
	}
	return nil
}

func signingPayload(s Session) ([]byte, error) {
	s.Signature = nil
	return json.Marshal(s)
}

func fingerprint(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}

func validateChangedFiles(files []gitstate.ChangedFile) error {
	for i, file := range files {
		prefix := fmt.Sprintf("git.changed_files[%d]", i)
		if strings.TrimSpace(file.Path) == "" {
			return fmt.Errorf("session %s.path is required", prefix)
		}
		if !safeSessionPath(file.Path) {
			return fmt.Errorf("session %s.path %q is unsafe", prefix, file.Path)
		}
		if strings.TrimSpace(file.Status) == "" {
			return fmt.Errorf("session %s.status is required", prefix)
		}
		if !file.ContentCaptured {
			continue
		}
		if strings.TrimSpace(file.ContentEncoding) == "" {
			return fmt.Errorf("session %s.content_encoding is required", prefix)
		}
		if file.ContentEncoding != "utf-8" {
			return fmt.Errorf("session %s.content_encoding %q is unsupported", prefix, file.ContentEncoding)
		}
		if strings.TrimSpace(file.ContentSHA256) == "" {
			return fmt.Errorf("session %s.content_sha256 is required", prefix)
		}
	}

	return nil
}

func safeSessionPath(path string) bool {
	cleanPath := filepath.Clean(path)
	return !filepath.IsAbs(cleanPath) && cleanPath != ".." && !strings.HasPrefix(cleanPath, "../")
}
