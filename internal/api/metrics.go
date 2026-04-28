package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler returns the Prometheus metrics scrape handler.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
