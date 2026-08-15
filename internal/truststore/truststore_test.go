package truststore

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	fingerprintA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fingerprintB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestAddStoresTrustedDevice(t *testing.T) {
	store, added, err := Add(Store{}, "desktop", fingerprintB, time.Date(2026, 8, 15, 23, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if !added {
		t.Fatal("added = false, want true")
	}
	if store.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", store.SchemaVersion, SchemaVersion)
	}
	if len(store.Devices) != 1 {
		t.Fatalf("device count = %d, want 1", len(store.Devices))
	}
	if store.Devices[0].TrustedAt != "2026-08-15T23:30:00Z" {
		t.Fatalf("trusted_at = %q", store.Devices[0].TrustedAt)
	}
}

func TestAddUpdatesExistingDeviceName(t *testing.T) {
	store, _, err := Add(Store{}, "desktop", fingerprintA, time.Now())
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	store, added, err := Add(store, "windows-pc", fingerprintA, time.Now())
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if added {
		t.Fatal("added = true, want false")
	}
	if len(store.Devices) != 1 {
		t.Fatalf("device count = %d, want 1", len(store.Devices))
	}
	if store.Devices[0].Name != "windows-pc" {
		t.Fatalf("name = %q, want windows-pc", store.Devices[0].Name)
	}
}

func TestRemoveTrustedDevice(t *testing.T) {
	store, _, err := Add(Store{}, "desktop", fingerprintA, time.Now())
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	store, removed, err := Remove(store, fingerprintA)
	if err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if !removed {
		t.Fatal("removed = false, want true")
	}
	if len(store.Devices) != 0 {
		t.Fatalf("device count = %d, want 0", len(store.Devices))
	}
}

func TestContainsFindsTrustedDevice(t *testing.T) {
	store, _, err := Add(Store{}, "desktop", fingerprintA, time.Now())
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	ok, err := Contains(store, strings.ToUpper(fingerprintA))
	if err != nil {
		t.Fatalf("Contains returned error: %v", err)
	}
	if !ok {
		t.Fatal("contains = false, want true")
	}
}

func TestContainsRejectsInvalidFingerprint(t *testing.T) {
	_, err := Contains(Store{}, "bad")
	if err == nil {
		t.Fatal("Contains returned nil error for invalid fingerprint")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	store, _, err := Add(Store{}, "desktop", fingerprintA, time.Now())
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	path := filepath.Join(t.TempDir(), ".staterelay", "trusted-devices.json")
	if err := Save(path, store); err != nil {
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
	if got.Devices[0].Fingerprint != fingerprintA {
		t.Fatalf("fingerprint = %q", got.Devices[0].Fingerprint)
	}
}

func TestSaveTightensExistingFilePermissions(t *testing.T) {
	store, _, err := Add(Store{}, "desktop", fingerprintA, time.Now())
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	path := filepath.Join(t.TempDir(), ".staterelay", "trusted-devices.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := Save(path, store); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestLoadMissingReturnsEmptyStore(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if store.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", store.SchemaVersion, SchemaVersion)
	}
	if len(store.Devices) != 0 {
		t.Fatalf("device count = %d, want 0", len(store.Devices))
	}
}

func TestReadJSONRejectsDuplicateFingerprint(t *testing.T) {
	input := `{
		"schema_version": 1,
		"devices": [
			{"name": "one", "fingerprint": "` + fingerprintA + `", "trusted_at": "2026-08-15T23:30:00Z"},
			{"name": "two", "fingerprint": "` + fingerprintA + `", "trusted_at": "2026-08-15T23:31:00Z"}
		]
	}`

	_, err := ReadJSON(strings.NewReader(input))
	if err == nil {
		t.Fatal("ReadJSON returned nil error for duplicate fingerprint")
	}
	if err.Error() != "trusted device "+fingerprintA+" is duplicated" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestWriteJSONRoundTrip(t *testing.T) {
	store, _, err := Add(Store{}, "desktop", fingerprintA, time.Now())
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, store); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
	got, err := ReadJSON(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if got.Devices[0].Name != "desktop" {
		t.Fatalf("name = %q", got.Devices[0].Name)
	}
}
