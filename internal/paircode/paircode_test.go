package paircode

import (
	"regexp"
	"testing"
)

const (
	testLocalFingerprint = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testPeerFingerprint  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestCodeIsStableAcrossFingerprintOrder(t *testing.T) {
	left, err := Code(testLocalFingerprint, testPeerFingerprint)
	if err != nil {
		t.Fatalf("Code returned error: %v", err)
	}
	right, err := Code(testPeerFingerprint, testLocalFingerprint)
	if err != nil {
		t.Fatalf("Code returned error: %v", err)
	}
	if left != right {
		t.Fatalf("code changed with order: %q != %q", left, right)
	}
	if !regexp.MustCompile(`^\d{3}-\d{3}$`).MatchString(left) {
		t.Fatalf("code format = %q", left)
	}
}

func TestCodeNormalizesFingerprintCase(t *testing.T) {
	code, err := Code(testLocalFingerprint, "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB")
	if err != nil {
		t.Fatalf("Code returned error: %v", err)
	}
	want, err := Code(testLocalFingerprint, testPeerFingerprint)
	if err != nil {
		t.Fatalf("Code returned error: %v", err)
	}
	if code != want {
		t.Fatalf("code = %q, want %q", code, want)
	}
}

func TestCodeRejectsInvalidFingerprint(t *testing.T) {
	_, err := Code(testLocalFingerprint, "not-a-fingerprint")
	if err == nil {
		t.Fatal("Code returned nil error for invalid fingerprint")
	}
}

func TestCodeRejectsSameFingerprint(t *testing.T) {
	_, err := Code(testLocalFingerprint, testLocalFingerprint)
	if err == nil {
		t.Fatal("Code returned nil error for matching fingerprints")
	}
}
