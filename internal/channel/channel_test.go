package channel

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestHashProviderUser_IsStableAndAppScoped(t *testing.T) {
	h1, err := HashProviderUser("secret", PlatformWeixin, "app-a", "openid-1")
	if err != nil {
		t.Fatalf("hash provider user: %v", err)
	}
	h2, err := HashProviderUser("secret", PlatformWeixin, "app-a", "openid-1")
	if err != nil {
		t.Fatalf("hash provider user again: %v", err)
	}
	h3, err := HashProviderUser("secret", PlatformWeixin, "app-b", "openid-1")
	if err != nil {
		t.Fatalf("hash provider user different app: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("same input hash mismatch: %q != %q", h1, h2)
	}
	if h1 == h3 {
		t.Fatal("hash should be scoped by app key")
	}
}

func TestChallengeStore_StateIsSingleUse(t *testing.T) {
	store := NewChallengeStore(time.Minute)
	ch, err := store.Create(PlatformFeishu, "app-1", "user-hash", "/home")
	if err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	state, err := store.CreateState(ch.ID)
	if err != nil {
		t.Fatalf("create state: %v", err)
	}
	taken, err := store.TakeState(state.ID)
	if err != nil {
		t.Fatalf("take state: %v", err)
	}
	if taken.ID != ch.ID {
		t.Fatalf("challenge id = %q, want %q", taken.ID, ch.ID)
	}
	if _, err := store.TakeState(state.ID); err == nil {
		t.Fatal("state reuse should fail")
	}
	if _, err := store.Peek(ch.ID); err == nil {
		t.Fatal("challenge should be consumed with state")
	}
}

func TestWebhookSignatures(t *testing.T) {
	token := "token"
	timestamp := "12345"
	nonce := "nonce"
	weixinSig := sha1Sorted(token, timestamp, nonce)
	if !VerifyWeixinSignature(token, timestamp, nonce, weixinSig) {
		t.Fatal("valid weixin signature rejected")
	}
	if VerifyWeixinSignature(token, timestamp, nonce, "bad") {
		t.Fatal("invalid weixin signature accepted")
	}

	encrypt := "ciphertext"
	wecomSig := sha1Sorted(token, timestamp, nonce, encrypt)
	if !VerifyWeComSignature(token, timestamp, nonce, encrypt, wecomSig) {
		t.Fatal("valid wecom signature rejected")
	}
	if VerifyWeComSignature(token, timestamp, nonce, encrypt, weixinSig) {
		t.Fatal("wecom signature should include encrypt payload")
	}
}

func sha1Sorted(parts ...string) string {
	sort.Strings(parts)
	h := sha1.New()
	_, _ = h.Write([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", h.Sum(nil))
}
