package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"notsofast/core"
	"strings"
	"sync/atomic"
	"time"
)

type Handler struct {
	s        *core.Service
	slots    chan struct{}
	requests atomic.Uint64
	rejected atomic.Uint64
}

func NewHandler(s *core.Service, workers int) *Handler {
	if workers < 1 || workers > 64 {
		workers = 4
	}
	return &Handler{s: s, slots: make(chan struct{}, workers)}
}
func writeError(w http.ResponseWriter, status int, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"reason": reason}})
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.requests.Add(1)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	auth := r.Header.Get("Authorization")
	user, ok := h.s.Authenticate(strings.TrimPrefix(auth, "Bearer "))
	if !ok || !strings.HasPrefix(auth, "Bearer ") {
		h.rejected.Add(1)
		writeError(w, 401, "UNAUTHENTICATED")
		return
	}
	if r.Header.Get("Origin") != "" {
		writeError(w, 403, "BROWSER_ORIGIN_DENIED")
		return
	}
	if r.URL.Path == "/metrics" && r.Method == "GET" {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "nsf_requests_total %d\nnsf_rejected_total %d\nnsf_active_requests %d\n", h.requests.Load(), h.rejected.Load(), len(h.slots))
		return
	}
	if r.Method != "POST" || !strings.HasPrefix(r.URL.Path, "/v1/") {
		writeError(w, 404, "NOT_FOUND")
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		h.rejected.Add(1)
		writeError(w, 429, "BUSY")
		return
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	defer r.Body.Close()
	body, e := io.ReadAll(r.Body)
	if e != nil {
		writeError(w, 413, "REQUEST_LIMIT")
		return
	}
	op := strings.TrimPrefix(r.URL.Path, "/v1/")
	result, e := Dispatch(ctx, h.s, user, op, body, r.URL.Query().Get("expand") == "1")
	code := core.Code(e)
	slog.Info("request", "reason", code, "duration_ms", time.Since(start).Milliseconds())
	if e != nil {
		h.rejected.Add(1)
		status := 400
		switch code {
		case "FORBIDDEN":
			status = 403
		case "NOT_FOUND":
			status = 404
		case "STATE_CHANGED", "OPERATION_CONFLICT", "POLICY_CHANGED":
			status = 409
		case "UNRESOLVED":
			status = 503
		case "STORAGE_LIMIT", "LIMIT":
			status = 507
		case "CANCELLED":
			status = 408
		case "INTERNAL", "GIT_ERROR":
			status = 500
		}
		writeError(w, status, code)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"result": result})
}
