package deviceidentity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const SchemaVersion = 1

type Identity struct {
	SchemaVersion  int    `json:"schema_version"`
	CreatedAt      string `json:"created_at"`
	Name           string `json:"name"`
	PublicKey      string `json:"public_key"`
	PrivateKeySeed string `json:"private_key_seed"`
	Fingerprint    string `json:"fingerprint"`
}

func Path(root string) string {
	return filepath.Join(root, ".staterelay", "device-identity.json")
}

func LoadOrCreate(path string, name string, now time.Time) (Identity, bool, error) {
	existing, err := Load(path)
	if err == nil {
		return existing, false, nil
	}
	if !os.IsNotExist(err) {
		return Identity{}, false, err
	}

	identity, err := New(name, now, rand.Reader)
	if err != nil {
		return Identity{}, false, err
	}
	if err := Save(path, identity); err != nil {
		return Identity{}, false, err
	}
	return identity, true, nil
}

func New(name string, createdAt time.Time, random io.Reader) (Identity, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Identity{}, fmt.Errorf("device name is required")
	}

	publicKey, privateKey, err := ed25519.GenerateKey(random)
	if err != nil {
		return Identity{}, fmt.Errorf("generate device key: %w", err)
	}

	identity := Identity{
		SchemaVersion:  SchemaVersion,
		CreatedAt:      createdAt.UTC().Format(time.RFC3339),
		Name:           name,
		PublicKey:      base64.StdEncoding.EncodeToString(publicKey),
		PrivateKeySeed: base64.StdEncoding.EncodeToString(privateKey.Seed()),
		Fingerprint:    Fingerprint(publicKey),
	}
	if err := Validate(identity); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func Load(path string) (Identity, error) {
	file, err := os.Open(path)
	if err != nil {
		return Identity{}, err
	}
	defer file.Close()

	var identity Identity
	if err := json.NewDecoder(file).Decode(&identity); err != nil {
		return Identity{}, err
	}
	if err := Validate(identity); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func Save(path string, identity Identity) error {
	if err := Validate(identity); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create identity directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(identity); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func Validate(identity Identity) error {
	if identity.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported device identity schema version %d", identity.SchemaVersion)
	}
	if strings.TrimSpace(identity.CreatedAt) == "" {
		return fmt.Errorf("device identity created_at is required")
	}
	if strings.TrimSpace(identity.Name) == "" {
		return fmt.Errorf("device identity name is required")
	}

	publicKey, err := base64.StdEncoding.DecodeString(identity.PublicKey)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("device identity public key has %d bytes, want %d", len(publicKey), ed25519.PublicKeySize)
	}
	seed, err := base64.StdEncoding.DecodeString(identity.PrivateKeySeed)
	if err != nil {
		return fmt.Errorf("decode private key seed: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("device identity private key seed has %d bytes, want %d", len(seed), ed25519.SeedSize)
	}
	if identity.Fingerprint != Fingerprint(publicKey) {
		return fmt.Errorf("device identity fingerprint does not match public key")
	}
	return nil
}

func Fingerprint(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}
