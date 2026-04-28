package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// DBPinger checks database connectivity.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves enhanced health probes.
type HealthHandler struct {
	db DBPinger
}

func NewHealthHandler(db DBPinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// LiveHandler returns 200 if the process is alive (Kubernetes liveness).
func (h *HealthHandler) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	}
}

// ReadyHandler returns 200 only if all dependencies are reachable (Kubernetes readiness).
func (h *HealthHandler) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]string{}
		healthy := true

		if h.db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := h.db.Ping(ctx); err != nil {
				checks["database"] = "unhealthy: " + err.Error()
				healthy = false
			} else {
				checks["database"] = "ok"
			}
		}

		status := http.StatusOK
		checks["status"] = "ready"
		if !healthy {
			status = http.StatusServiceUnavailable
			checks["status"] = "not_ready"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(checks)
	}
}
