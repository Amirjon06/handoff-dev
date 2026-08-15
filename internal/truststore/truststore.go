package truststore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const SchemaVersion = 1

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Device struct {
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	TrustedAt   string `json:"trusted_at"`
}

type Store struct {
	SchemaVersion int      `json:"schema_version"`
	Devices       []Device `json:"devices,omitempty"`
}

func Path(root string) string {
	return filepath.Join(root, ".staterelay", "trusted-devices.json")
}

func Load(path string) (Store, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Store{SchemaVersion: SchemaVersion}, nil
	}
	if err != nil {
		return Store{}, err
	}
	defer file.Close()

	return ReadJSON(file)
}

func Save(path string, store Store) error {
	if err := Validate(store); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create trust store directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}

	if err := WriteJSON(file, store); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func Add(store Store, name string, fingerprint string, trustedAt time.Time) (Store, bool, error) {
	if store.SchemaVersion == 0 {
		store.SchemaVersion = SchemaVersion
	}
	name = strings.TrimSpace(name)
	fingerprint = strings.ToLower(strings.TrimSpace(fingerprint))
	device := Device{
		Name:        name,
		Fingerprint: fingerprint,
		TrustedAt:   trustedAt.UTC().Format(time.RFC3339),
	}
	if err := validateDevice(device, 0); err != nil {
		return Store{}, false, err
	}

	for i, existing := range store.Devices {
		if existing.Fingerprint == fingerprint {
			store.Devices[i].Name = name
			if err := Validate(store); err != nil {
				return Store{}, false, err
			}
			return store, false, nil
		}
	}
	store.Devices = append(store.Devices, device)
	sortDevices(store.Devices)
	if err := Validate(store); err != nil {
		return Store{}, false, err
	}
	return store, true, nil
}

func Remove(store Store, fingerprint string) (Store, bool, error) {
	if store.SchemaVersion == 0 {
		store.SchemaVersion = SchemaVersion
	}
	fingerprint = strings.ToLower(strings.TrimSpace(fingerprint))
	if !fingerprintPattern.MatchString(fingerprint) {
		return Store{}, false, fmt.Errorf("trusted device fingerprint must be 64 lowercase hex characters")
	}

	devices := store.Devices[:0]
	removed := false
	for _, device := range store.Devices {
		if device.Fingerprint == fingerprint {
			removed = true
			continue
		}
		devices = append(devices, device)
	}
	store.Devices = devices
	if err := Validate(store); err != nil {
		return Store{}, false, err
	}
	return store, removed, nil
}

func ReadJSON(r io.Reader) (Store, error) {
	var store Store
	if err := json.NewDecoder(r).Decode(&store); err != nil {
		return Store{}, err
	}
	if err := Validate(store); err != nil {
		return Store{}, err
	}
	return store, nil
}

func WriteJSON(w io.Writer, store Store) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(store)
}

func Validate(store Store) error {
	if store.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported trust store schema version %d", store.SchemaVersion)
	}
	seen := make(map[string]bool, len(store.Devices))
	for i, device := range store.Devices {
		if err := validateDevice(device, i); err != nil {
			return err
		}
		if seen[device.Fingerprint] {
			return fmt.Errorf("trusted device %s is duplicated", device.Fingerprint)
		}
		seen[device.Fingerprint] = true
	}
	return nil
}

func validateDevice(device Device, index int) error {
	prefix := fmt.Sprintf("trusted devices[%d]", index)
	if strings.TrimSpace(device.Name) == "" {
		return fmt.Errorf("%s.name is required", prefix)
	}
	if !fingerprintPattern.MatchString(device.Fingerprint) {
		return fmt.Errorf("%s.fingerprint must be 64 lowercase hex characters", prefix)
	}
	if strings.TrimSpace(device.TrustedAt) == "" {
		return fmt.Errorf("%s.trusted_at is required", prefix)
	}
	return nil
}

func sortDevices(devices []Device) {
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Name == devices[j].Name {
			return devices[i].Fingerprint < devices[j].Fingerprint
		}
		return devices[i].Name < devices[j].Name
	})
}
