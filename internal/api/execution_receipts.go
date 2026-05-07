package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type ExecutionReceiptHandler struct {
	store store.ExecutionReceiptStore
}

func NewExecutionReceiptHandler(s store.ExecutionReceiptStore) *ExecutionReceiptHandler {
	return &ExecutionReceiptHandler{store: s}
}

func (h *ExecutionReceiptHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/execution-receipts")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		h.list(w, r)
	case r.Method == http.MethodGet && path != "":
		h.get(w, r, path)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ExecutionReceiptHandler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromContext(r.Context())
	if tenantID == "" {
		http.Error(w, "tenant context required", http.StatusBadRequest)
		return
	}

	opts := store.ReceiptListOptions{
		SessionID: r.URL.Query().Get("session_id"),
		ToolName:  r.URL.Query().Get("tool_name"),
		Status:    r.URL.Query().Get("status"),
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			opts.Offset = n
		}
	}

	receipts, total, err := h.store.List(r.Context(), tenantID, opts)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"execution_receipts": receipts,
		"total":              total,
	})
}

func (h *ExecutionReceiptHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	tenantID := middleware.TenantFromContext(r.Context())
	if tenantID == "" {
		http.Error(w, "tenant context required", http.StatusBadRequest)
		return
	}

	receipt, err := h.store.Get(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(receipt)
}
