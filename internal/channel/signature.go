package channel

import (
	"crypto/sha1" //nolint:gosec // WeChat/WeCom platform API mandates SHA1 for callback signature verification
	"crypto/subtle"
	"fmt"
	"sort"
	"strings"
)

// VerifyWeixinSignature verifies a WeChat callback signature.
// SHA1 is required by the WeChat platform API specification.
func VerifyWeixinSignature(token, timestamp, nonce, signature string) bool {
	if token == "" || timestamp == "" || nonce == "" || signature == "" {
		return false
	}
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	h := sha1.New() //nolint:gosec // WeChat API requires SHA1
	_, _ = h.Write([]byte(strings.Join(parts, "")))
	computed := fmt.Sprintf("%x", h.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(computed), []byte(signature)) == 1
}

// VerifyWeComSignature verifies a WeCom callback signature.
// SHA1 is required by the WeCom platform API specification.
func VerifyWeComSignature(token, timestamp, nonce, encrypt, signature string) bool {
	if token == "" || timestamp == "" || nonce == "" || signature == "" {
		return false
	}
	parts := []string{token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	h := sha1.New() //nolint:gosec // WeCom API requires SHA1
	_, _ = h.Write([]byte(strings.Join(parts, "")))
	computed := fmt.Sprintf("%x", h.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(computed), []byte(signature)) == 1
}
