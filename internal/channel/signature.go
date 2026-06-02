package channel

import (
	"crypto/sha1"
	"crypto/subtle"
	"fmt"
	"sort"
	"strings"
)

func VerifyWeixinSignature(token, timestamp, nonce, signature string) bool {
	if token == "" || timestamp == "" || nonce == "" || signature == "" {
		return false
	}
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	h := sha1.New()
	_, _ = h.Write([]byte(strings.Join(parts, "")))
	computed := fmt.Sprintf("%x", h.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(computed), []byte(signature)) == 1
}

func VerifyWeComSignature(token, timestamp, nonce, encrypt, signature string) bool {
	if token == "" || timestamp == "" || nonce == "" || signature == "" {
		return false
	}
	parts := []string{token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	h := sha1.New()
	_, _ = h.Write([]byte(strings.Join(parts, "")))
	computed := fmt.Sprintf("%x", h.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(computed), []byte(signature)) == 1
}
