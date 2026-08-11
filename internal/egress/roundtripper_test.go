package egress

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type captureRoundTripper struct {
	tenantID string
	path     string
}

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.tenantID = TenantFromContext(req.Context())
	c.path = PathFromContext(req.Context())
	return &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

func TestTenantAwareRoundTripper_InjectsTenantAndPath(t *testing.T) {
	base := &captureRoundTripper{}
	rt := NewTenantAwareRoundTripper(base, "tenant-1")

	req, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/chat?x=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()

	if base.tenantID != "tenant-1" {
		t.Fatalf("tenant in transport context = %q, want tenant-1", base.tenantID)
	}
	if base.path != "/v1/chat" {
		t.Fatalf("path in transport context = %q, want /v1/chat", base.path)
	}
}

func TestValidateRedirectTarget_NilRequest(t *testing.T) {
	if err := ValidateRedirectTarget(nil); err != ErrNotAllowed {
		t.Errorf("nil request: got %v, want ErrNotAllowed", err)
	}
}

func TestValidateRedirectTarget_NilURL(t *testing.T) {
	req := &http.Request{}
	if err := ValidateRedirectTarget(req); err != ErrNotAllowed {
		t.Errorf("nil URL: got %v, want ErrNotAllowed", err)
	}
}

func TestValidateRedirectTarget_InvalidScheme(t *testing.T) {
	req := &http.Request{URL: &url.URL{Scheme: "ftp", Host: "example.com"}}
	if err := ValidateRedirectTarget(req); err != ErrNotAllowed {
		t.Errorf("ftp scheme: got %v, want ErrNotAllowed", err)
	}
}

func TestValidateRedirectTarget_EmptyHost(t *testing.T) {
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: ""}}
	if err := ValidateRedirectTarget(req); err != ErrNotAllowed {
		t.Errorf("empty host: got %v, want ErrNotAllowed", err)
	}
}

func TestValidateRedirectTarget_BlockedIP(t *testing.T) {
	// 127.0.0.1 is a loopback/blocked IP.
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "127.0.0.1"}}
	if err := ValidateRedirectTarget(req); err != ErrBlockedIP {
		t.Errorf("blocked IP: got %v, want ErrBlockedIP", err)
	}
}

func TestValidateRedirectTarget_ValidHTTPS(t *testing.T) {
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "api.example.com"}}
	if err := ValidateRedirectTarget(req); err != nil {
		t.Errorf("valid https: got %v, want nil", err)
	}
}

func TestValidateRedirectTarget_ValidHTTP(t *testing.T) {
	req := &http.Request{URL: &url.URL{Scheme: "http", Host: "api.example.com"}}
	if err := ValidateRedirectTarget(req); err != nil {
		t.Errorf("valid http: got %v, want nil", err)
	}
}
