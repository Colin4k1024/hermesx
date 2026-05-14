package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"
)

// StartAdminServer starts a pprof HTTP server bound to 127.0.0.1 only.
// If HERMESX_ADMIN_TOKEN is set, all requests must carry "Authorization: Bearer <token>".
// Only called when HERMESX_ADMIN_PORT is non-empty; blocks until the server exits.
func StartAdminServer(port string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	var handler http.Handler = mux
	if token := os.Getenv("HERMESX_ADMIN_TOKEN"); token != "" {
		handler = bearerAuth(token, mux)
	}

	addr := "127.0.0.1:" + strings.TrimPrefix(port, ":")
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	slog.Info("admin server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("admin server stopped", "error", err)
	}
}

func bearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := []byte("Bearer " + token)
		actual := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(actual, expected) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
