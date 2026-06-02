package channel

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HashProviderUser produces a stable tenant-safe lookup key for a provider user id.
func HashProviderUser(secret, platform, appKey, providerUserID string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("channel hash secret is required")
	}
	if platform == "" || appKey == "" || providerUserID == "" {
		return "", fmt.Errorf("platform, app_key, and provider user id are required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(platform))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(appKey))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(providerUserID))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func RandomToken(prefix string, bytesLen int) (string, error) {
	if bytesLen <= 0 {
		bytesLen = 32
	}
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return prefix + hex.EncodeToString(b), nil
}
