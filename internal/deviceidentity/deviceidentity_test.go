package deviceidentity

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCreatesValidIdentity(t *testing.T) {
	random := bytes.NewReader(bytes.Repeat([]byte{7}, 32))
	identity, err := New("amir-macbook", time.Date(2026, 8, 15, 23, 0, 0, 0, time.UTC), random)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if identity.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", identity.SchemaVersion, SchemaVersion)
	}
	if identity.CreatedAt != "2026-08-15T23:00:00Z" {
		t.Fatalf("created_at = %q", identity.CreatedAt)
	}
	if identity.Name != "amir-macbook" {
		t.Fatalf("name = %q", identity.Name)
	}
	if identity.Fingerprint == "" {
		t.Fatal("fingerprint is empty")
	}
	if err := Validate(identity); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	random := bytes.NewReader(bytes.Repeat([]byte{9}, 32))
	identity, err := New("desktop", time.Date(2026, 8, 15, 23, 0, 0, 0, time.UTC), random)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	path := filepath.Join(t.TempDir(), ".staterelay", "device-identity.json")
	if err := Save(path, identity); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.Fingerprint != identity.Fingerprint {
		t.Fatalf("fingerprint = %q, want %q", got.Fingerprint, identity.Fingerprint)
	}
}

func TestLoadOrCreateKeepsExistingIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".staterelay", "device-identity.json")
	first, created, err := LoadOrCreate(path, "first", time.Date(2026, 8, 15, 23, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("LoadOrCreate returned error: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}

	second, created, err := LoadOrCreate(path, "second", time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("LoadOrCreate returned error: %v", err)
	}
	if created {
		t.Fatal("created = true, want false")
	}
	if second.Fingerprint != first.Fingerprint {
		t.Fatalf("fingerprint changed from %q to %q", first.Fingerprint, second.Fingerprint)
	}
	if second.Name != "first" {
		t.Fatalf("name = %q, want first", second.Name)
	}
}

func TestValidateRejectsFingerprintMismatch(t *testing.T) {
	random := bytes.NewReader(bytes.Repeat([]byte{4}, 32))
	identity, err := New("mac", time.Now(), random)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	identity.Fingerprint = "bad"

	err = Validate(identity)
	if err == nil {
		t.Fatal("Validate returned nil error for fingerprint mismatch")
	}
	if err.Error() != "device identity fingerprint does not match public key" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestValidateRejectsBadSeedLength(t *testing.T) {
	random := bytes.NewReader(bytes.Repeat([]byte{4}, 32))
	identity, err := New("mac", time.Now(), random)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	identity.PrivateKeySeed = base64.StdEncoding.EncodeToString([]byte{1, 2, 3})

	err = Validate(identity)
	if err == nil {
		t.Fatal("Validate returned nil error for bad seed length")
	}
	if err.Error() != "device identity private key seed has 3 bytes, want 32" {
		t.Fatalf("error = %q", err.Error())
	}
}
