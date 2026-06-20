package security

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode"
)

func TestGenerateSessionToken(t *testing.T) {
	token, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(token, "tl_sess_") {
		t.Fatalf("token prefix mismatch: %s", token)
	}

	encoded := strings.TrimPrefix(token, "tl_sess_")
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("token is not valid base64 raw url encoding: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("expected 32 random bytes, got %d", len(decoded))
	}
}

func TestGenerateEmailCode(t *testing.T) {
	code, err := GenerateEmailCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(code) != 6 {
		t.Fatalf("expected 6 digits, got %q", code)
	}
	for _, r := range code {
		if !unicode.IsDigit(r) {
			t.Fatalf("code must contain digits only: %q", code)
		}
	}
}

func TestHashAndVerifySecret(t *testing.T) {
	secret := "secret-value"
	pepper := "pepper-value"

	hash := HashSecret(secret, pepper)
	if hash == "" {
		t.Fatalf("expected hash")
	}
	if !VerifySecret(secret, pepper, hash) {
		t.Fatalf("expected secret to verify")
	}
	if VerifySecret(secret+"x", pepper, hash) {
		t.Fatalf("expected wrong secret to fail verification")
	}
}
