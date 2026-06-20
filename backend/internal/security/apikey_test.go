package security

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey("live")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(key, "tl_live_") {
		t.Fatalf("key prefix mismatch: %s", key)
	}

	display := DisplayParts(key)
	if display.Prefix == "" {
		t.Fatalf("expected display prefix")
	}
	if len(display.Last4) != 4 {
		t.Fatalf("expected last4 length 4, got %q", display.Last4)
	}
}

func TestDisplayPartsShortString(t *testing.T) {
	display := DisplayParts("abc")
	if display.Prefix != "abc" {
		t.Fatalf("prefix mismatch: %q", display.Prefix)
	}
	if display.Last4 != "abc" {
		t.Fatalf("last4 mismatch: %q", display.Last4)
	}
}

func TestHashAndVerifyAPIKey(t *testing.T) {
	key := "tl_live_example_secret"
	pepper := "test-pepper"

	hash := HashAPIKey(key, pepper)
	if hash == "" {
		t.Fatalf("expected hash")
	}
	if strings.Contains(hash, key) {
		t.Fatalf("hash must not contain plaintext key")
	}
	if !VerifyAPIKey(key, pepper, hash) {
		t.Fatalf("expected key to verify")
	}
	if VerifyAPIKey(key+"x", pepper, hash) {
		t.Fatalf("expected modified key to fail verification")
	}
}
