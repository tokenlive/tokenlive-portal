package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

type APIKeyDisplay struct {
	Prefix string
	Last4  string
}

func GenerateAPIKey(environment string) (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate api key random bytes: %w", err)
	}

	secret := base64.RawURLEncoding.EncodeToString(random)
	return fmt.Sprintf("tl_%s_%s", environment, secret), nil
}

func DisplayParts(key string) APIKeyDisplay {
	prefixLen := 12
	if len(key) < prefixLen {
		prefixLen = len(key)
	}

	lastLen := 4
	if len(key) < lastLen {
		lastLen = len(key)
	}

	return APIKeyDisplay{
		Prefix: key[:prefixLen],
		Last4:  key[len(key)-lastLen:],
	}
}

func HashAPIKey(key string, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(key))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyAPIKey(key string, pepper string, expectedHash string) bool {
	actualHash := HashAPIKey(key, pepper)
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}
