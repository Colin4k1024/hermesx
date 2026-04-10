package tools

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"
)

// activeBackend is the singleton browser backend.
var (
	activeBackend   BrowserBackend
	activeBackendMu sync.Mutex
)

// getOrCreateBackend returns the active browser backend, creating one if needed.
// Backend selection priority:
//  1. BROWSER_BACKEND env var ("local" or "browserbase")
//  2. BROWSERBASE_API_KEY set → browserbase
//  3. BROWSER_CDP_URL set or Chrome available → local
//  4. Error: no backend available
func getOrCreateBackend() (BrowserBackend, error) {
	activeBackendMu.Lock()
	defer activeBackendMu.Unlock()

	if activeBackend != nil {
		return activeBackend, nil
	}

	backend, err := selectBackend()
	if err != nil {
		return nil, err
	}

	if err := backend.Connect(); err != nil {
		return nil, fmt.Errorf("connect %s backend: %w", backend.Name(), err)
	}

	activeBackend = backend
	return activeBackend, nil
}

func selectBackend() (BrowserBackend, error) {
	// Explicit selection via env var.
	explicit := os.Getenv("BROWSER_BACKEND")
	switch explicit {
	case "local":
		return &LocalBrowserBackend{}, nil
	case "browserbase":
		if os.Getenv("BROWSERBASE_API_KEY") == "" {
			return nil, fmt.Errorf("BROWSER_BACKEND=browserbase but BROWSERBASE_API_KEY not set")
		}
		return &BrowserbaseBackend{}, nil
	case "":
		// Auto-detect below.
	default:
		return nil, fmt.Errorf("unknown BROWSER_BACKEND %q (supported: local, browserbase)", explicit)
	}

	// Auto-detect: prefer browserbase if configured, fall back to local.
	if os.Getenv("BROWSERBASE_API_KEY") != "" {
		return &BrowserbaseBackend{}, nil
	}

	// Try local (Chrome/Chromium).
	if os.Getenv("BROWSER_CDP_URL") != "" || findChromeBinary() != "" {
		return &LocalBrowserBackend{}, nil
	}

	return nil, fmt.Errorf(
		"no browser backend available. Options:\n" +
			"  1. Set BROWSERBASE_API_KEY for cloud browsing (browserbase.com)\n" +
			"  2. Install Chrome/Chromium for local browsing\n" +
			"  3. Set BROWSER_CDP_URL to connect to an existing Chrome instance")
}

// closeActiveBackend closes the active browser backend.
func closeActiveBackend() {
	activeBackendMu.Lock()
	defer activeBackendMu.Unlock()

	if activeBackend != nil {
		activeBackend.Close()
		activeBackend = nil
	}
}

// checkNavigationSafety validates a URL is safe for browser navigation.
// Returns a reason string if blocked, empty string if safe.
func checkNavigationSafety(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "invalid url"
	}

	// Scheme whitelist.
	switch parsed.Scheme {
	case "http", "https":
		// allowed
	case "":
		return "missing url scheme"
	default:
		return fmt.Sprintf("blocked scheme: %s", parsed.Scheme)
	}

	hostname := parsed.Hostname()

	// Block cloud metadata endpoints.
	metadataHosts := []string{
		"169.254.169.254",
		"metadata.google.internal",
		"metadata.goog",
	}
	for _, mh := range metadataHosts {
		if hostname == mh {
			return "cloud metadata endpoint blocked"
		}
	}

	// Block localhost/loopback — except when targeting our own CDP.
	ips, err := net.LookupHost(hostname)
	if err != nil {
		// DNS failure — block to be safe (fail closed).
		return "dns resolution failed"
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return "loopback/link-local address blocked"
		}
		if ip.IsPrivate() {
			return "private network address blocked"
		}
	}

	return ""
}
