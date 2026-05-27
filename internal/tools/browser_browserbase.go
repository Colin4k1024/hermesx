package tools

import (
	"context"
	"os"
)

// BrowserbaseBackend wraps the existing BrowserSession to implement BrowserBackend.
type BrowserbaseBackend struct {
	session *BrowserSession
}

// Compile-time interface check.
var _ BrowserBackend = (*BrowserbaseBackend)(nil)

func (b *BrowserbaseBackend) Name() string { return "browserbase" }

func (b *BrowserbaseBackend) Connect(ctx context.Context, tctx *ToolContext) error {
	// Resolve API key: prefer SecretResolver, fall back to environment variable.
	apiKey := os.Getenv("BROWSERBASE_API_KEY") // fallback: use SecretResolver when available
	if tctx != nil && tctx.SecretResolver != nil {
		if k, err := tctx.SecretResolver.Resolve(ctx, "BROWSERBASE_API_KEY"); err == nil && k != "" {
			apiKey = k
		}
	}

	// Resolve project ID: prefer SecretResolver, fall back to environment variable.
	projectID := os.Getenv("BROWSERBASE_PROJECT_ID") // fallback: use SecretResolver when available
	if tctx != nil && tctx.SecretResolver != nil {
		if id, err := tctx.SecretResolver.Resolve(ctx, "BROWSERBASE_PROJECT_ID"); err == nil && id != "" {
			projectID = id
		}
	}

	session, err := newBrowserbaseSessionWithCreds(apiKey, projectID)
	if err != nil {
		return err
	}
	b.session = session
	return nil
}

func (b *BrowserbaseBackend) Close() {
	if b.session != nil {
		b.session.close()
	}
}

func (b *BrowserbaseBackend) Navigate(url string) (map[string]any, error) {
	return b.session.navigate(url)
}

func (b *BrowserbaseBackend) Snapshot() (map[string]any, error) {
	return b.session.snapshot()
}

func (b *BrowserbaseBackend) Click(ref string) (map[string]any, error) {
	return b.session.click(ref)
}

func (b *BrowserbaseBackend) Type(ref, text string, clearFirst bool) (map[string]any, error) {
	return b.session.typeText(ref, text, clearFirst)
}

func (b *BrowserbaseBackend) Scroll(direction string, amount int) (map[string]any, error) {
	return b.session.scroll(direction, amount)
}

func (b *BrowserbaseBackend) GoBack() (map[string]any, error) {
	return b.session.goBack()
}

func (b *BrowserbaseBackend) PressKey(key string) (map[string]any, error) {
	return b.session.pressKey(key)
}

func (b *BrowserbaseBackend) GetImages() (map[string]any, error) {
	return b.session.getImages()
}

func (b *BrowserbaseBackend) ExecuteScript(script string) (map[string]any, error) {
	return b.session.executeScript(script)
}

func (b *BrowserbaseBackend) CurrentURL() string {
	if b.session == nil {
		return ""
	}
	return b.session.currentURL
}

func (b *BrowserbaseBackend) PageTitle() string {
	if b.session == nil {
		return ""
	}
	return b.session.pageTitle
}
