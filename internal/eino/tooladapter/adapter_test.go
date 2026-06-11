package tooladapter

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/egress"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

type adapterObjectStore struct{}

func (adapterObjectStore) EnsureBucket(context.Context) error { return nil }
func (adapterObjectStore) Bucket() string                     { return "adapter-test" }
func (adapterObjectStore) Ping(context.Context) error         { return nil }
func (adapterObjectStore) GetObject(context.Context, string) ([]byte, error) {
	return nil, nil
}
func (adapterObjectStore) PutObject(context.Context, string, []byte) error { return nil }
func (adapterObjectStore) DeleteObject(context.Context, string) error      { return nil }
func (adapterObjectStore) ObjectExists(context.Context, string) (bool, error) {
	return false, nil
}
func (adapterObjectStore) ListObjects(context.Context, string) ([]string, error) {
	return nil, nil
}

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

func TestWrappedTool_UsesBaseContextWhenRuntimeContextIsMissing(t *testing.T) {
	var captured *tools.ToolContext
	entry := &tools.ToolEntry{
		Name: "capture_context",
		Schema: map[string]any{
			"name":        "capture_context",
			"description": "capture context",
			"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
		},
		Handler: func(_ context.Context, _ map[string]any, tctx *tools.ToolContext) string {
			copied := *tctx
			captured = &copied
			return `{"ok":true}`
		},
	}
	base := &tools.ToolContext{
		TenantID:    "tenant-1",
		UserID:      "user-1",
		ObjectStore: adapterObjectStore{},
	}
	wrapped := WrapWithRecorderAndContext(entry, nil, base)

	if _, err := wrapped.InvokableRun(context.Background(), `{}`); err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if captured == nil {
		t.Fatal("expected tool context to be captured")
	}
	if captured.TenantID != "tenant-1" || captured.UserID != "user-1" || captured.ObjectStore == nil {
		t.Fatalf("unexpected captured context: %#v", captured)
	}
}
