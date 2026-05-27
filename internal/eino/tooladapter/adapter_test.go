package tooladapter

import (
	"errors"
	"net/http"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/egress"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

func TestEnrichToolContext_RedirectGuardBlocksPrivateIPLiteral(t *testing.T) {
	tctx := &tools.ToolContext{
		TenantID: "tenant-1",
		Extra: map[string]any{
			"egress_transport": &http.Transport{},
		},
	}
	enriched := enrichToolContext(tctx, &tools.ToolEntry{MaxRedirects: 2})
	if enriched.HTTPClient == nil || enriched.HTTPClient.CheckRedirect == nil {
		t.Fatal("expected governed HTTP client with redirect guard")
	}

	req, err := http.NewRequest(http.MethodGet, "https://127.0.0.1/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := enriched.HTTPClient.CheckRedirect(req, nil); !errors.Is(err, egress.ErrBlockedIP) {
		t.Fatalf("redirect to loopback = %v, want ErrBlockedIP", err)
	}
}
