package paircode

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func Code(localFingerprint string, peerFingerprint string) (string, error) {
	local, err := normalizeFingerprint(localFingerprint)
	if err != nil {
		return "", fmt.Errorf("local fingerprint: %w", err)
	}
	peer, err := normalizeFingerprint(peerFingerprint)
	if err != nil {
		return "", fmt.Errorf("peer fingerprint: %w", err)
	}
	if local == peer {
		return "", fmt.Errorf("peer fingerprint matches local fingerprint")
	}

	fingerprints := []string{local, peer}
	sort.Strings(fingerprints)

	sum := sha256.Sum256([]byte("staterelay-pair-code-v1\n" + fingerprints[0] + "\n" + fingerprints[1]))
	value := binary.BigEndian.Uint32(sum[:4]) % 1_000_000
	return fmt.Sprintf("%03d-%03d", value/1000, value%1000), nil
}

func normalizeFingerprint(fingerprint string) (string, error) {
	fingerprint = strings.ToLower(strings.TrimSpace(fingerprint))
	if !fingerprintPattern.MatchString(fingerprint) {
		return "", fmt.Errorf("fingerprint must be 64 hex characters")
	}
	return fingerprint, nil
}
