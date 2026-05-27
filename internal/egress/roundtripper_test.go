package egress

import (
	"io"
	"net/http"
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
