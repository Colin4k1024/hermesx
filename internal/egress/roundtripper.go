package egress

import (
	"net"
	"net/http"
	"strings"
)

type tenantAwareRoundTripper struct {
	base     http.RoundTripper
	tenantID string
}

// NewTenantAwareRoundTripper injects tenant and request-path metadata before
// SecureTransport dials, including for requests built without WithContext.
func NewTenantAwareRoundTripper(base http.RoundTripper, tenantID string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &tenantAwareRoundTripper{base: base, tenantID: tenantID}
}

func (t *tenantAwareRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return t.base.RoundTrip(req)
	}
	path := "/"
	if req.URL != nil {
		path = req.URL.EscapedPath()
		if path == "" {
			path = "/"
		}
	}
	ctx := WithTenant(req.Context(), t.tenantID)
	ctx = WithPath(ctx, path)
	return t.base.RoundTrip(req.WithContext(ctx))
}

// ValidateRedirectTarget rejects redirect targets that should never be followed
// before the next RoundTrip has a chance to dial them.
func ValidateRedirectTarget(req *http.Request) error {
	if req == nil || req.URL == nil {
		return ErrNotAllowed
	}
	scheme := strings.ToLower(req.URL.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrNotAllowed
	}
	host := req.URL.Hostname()
	if host == "" {
		return ErrNotAllowed
	}
	if ip := net.ParseIP(host); ip != nil && IsBlockedIP(ip) {
		return ErrBlockedIP
	}
	return nil
}
