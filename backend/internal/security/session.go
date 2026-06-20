package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
)

const SessionCookieName = "tl_session"

func GenerateSessionToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate session token random bytes: %w", err)
	}

	return "tl_sess_" + base64.RawURLEncoding.EncodeToString(random), nil
}

func GenerateEmailCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generate email code: %w", err)
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}

func HashSecret(secret string, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifySecret(secret string, pepper string, expectedHash string) bool {
	actualHash := HashSecret(secret, pepper)
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}
