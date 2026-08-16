package tlsidentity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/deviceidentity"
)

func Certificate(identity deviceidentity.Identity, now time.Time) (tls.Certificate, error) {
	if err := deviceidentity.Validate(identity); err != nil {
		return tls.Certificate{}, err
	}
	publicKey, err := base64.StdEncoding.DecodeString(identity.PublicKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("decode public key: %w", err)
	}
	seed, err := base64.StdEncoding.DecodeString(identity.PrivateKeySeed)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("decode private key seed: %w", err)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	if !privateKey.Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(publicKey)) {
		return tls.Certificate{}, fmt.Errorf("device identity key pair does not match")
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber(identity.Fingerprint),
		Subject:      pkixName(identity.Name),
		NotBefore:    now.UTC().Add(-time.Hour),
		NotAfter:     now.UTC().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{identity.Name, "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, ed25519.PublicKey(publicKey), privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create TLS certificate: %w", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  privateKey,
		Leaf:        template,
	}, nil
}

func Fingerprint(cert *x509.Certificate) (string, error) {
	publicKey, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("certificate public key is %T, want ed25519.PublicKey", cert.PublicKey)
	}
	return deviceidentity.Fingerprint(publicKey), nil
}

func serialNumber(fingerprint string) *big.Int {
	sum := sha256.Sum256([]byte(fingerprint))
	return new(big.Int).SetBytes(sum[:16])
}

func pkixName(name string) pkix.Name {
	return pkix.Name{CommonName: name}
}
