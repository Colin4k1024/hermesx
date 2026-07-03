package platforms

import (
	"net/http"
	"time"
)

// platformHTTPClient is a shared HTTP client with sensible timeouts
// for all outbound platform API calls.
var platformHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}
