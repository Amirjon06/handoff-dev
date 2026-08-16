package tlsidentity

import (
	"bytes"
	"crypto/x509"
	"testing"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/deviceidentity"
)

func TestCertificateBuildsEd25519Certificate(t *testing.T) {
	identity, err := deviceidentity.New("test-mac", time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC), bytes.NewReader(bytes.Repeat([]byte{5}, 32)))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	cert, err := Certificate(identity, time.Date(2026, 8, 16, 13, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Certificate returned error: %v", err)
	}
	if len(cert.Certificate) != 1 {
		t.Fatalf("certificate chain length = %d, want 1", len(cert.Certificate))
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate returned error: %v", err)
	}
	if parsed.Subject.CommonName != "test-mac" {
		t.Fatalf("common name = %q", parsed.Subject.CommonName)
	}
	if len(parsed.ExtKeyUsage) != 2 {
		t.Fatalf("extended key usage count = %d", len(parsed.ExtKeyUsage))
	}
}

func TestCertificateRejectsInvalidIdentity(t *testing.T) {
	_, err := Certificate(deviceidentity.Identity{}, time.Now())
	if err == nil {
		t.Fatal("Certificate returned nil error for invalid identity")
	}
}
