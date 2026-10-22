package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
)

// invalidError is a validation failure carrying a safe, client-facing message.
type invalidError struct{ msg string }

func (e *invalidError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &invalidError{msg: fmt.Sprintf(format, args...)}
}

// errUnauthorized is a missing, invalid or disabled API key.
var errUnauthorized = errors.New("httpapi: unauthorized")

const internalMessage = "internal error"

// writeError is the only place an error envelope is produced (AD-8).
func writeError(w http.ResponseWriter, err error) {
	status, code, msg := http.StatusInternalServerError, "internal_error", internalMessage
	var inv *invalidError
	switch {
	case errors.As(err, &inv):
		status, code, msg = http.StatusBadRequest, "invalid_request", inv.msg
	case errors.Is(err, errUnauthorized):
		status, code, msg = http.StatusUnauthorized, "unauthorized", "missing or invalid API key"
	case errors.Is(err, store.ErrNotFound):
		status, code, msg = http.StatusNotFound, "not_found", "not found"
	case errors.Is(err, queue.ErrQueueFull):
		status, code, msg = http.StatusServiceUnavailable, "queue_full", "queue is full, retry later"
	case errors.Is(err, queue.ErrClosed):
		status, code, msg = http.StatusServiceUnavailable, "queue_full", "service shutting down"
	default:
		slog.Error("internal_error", "error_class", fmt.Sprintf("%T", err))
	}
	writeEnvelope(w, status, code, msg)
}

func writeEnvelope(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]map[string]string{
		"error": {"code": code, "message": msg},
	})
}

// handler adapts an error-returning handler to http.Handler.
func (s *server) handler(fn func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			writeError(w, err)
		}
	})
}

// captureWriter records status and headers and drops the body.
type captureWriter struct {
	h      http.Header
	status int
}

func (c *captureWriter) Header() http.Header         { return c.h }
func (c *captureWriter) WriteHeader(status int)      { c.status = status }
func (c *captureWriter) Write(b []byte) (int, error) { return len(b), nil }

// envelopeFallbacks replaces ServeMux's plain-text 404/405 with the envelope
// (405 maps to invalid_request and keeps Allow).
func envelopeFallbacks(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern == "" {
			cw := &captureWriter{h: http.Header{}, status: http.StatusOK}
			mux.ServeHTTP(cw, r)
			switch cw.status {
			case http.StatusNotFound:
				writeEnvelope(w, http.StatusNotFound, "not_found", "not found")
				return
			case http.StatusMethodNotAllowed:
				if allow := cw.h.Get("Allow"); allow != "" {
					w.Header().Set("Allow", allow)
				}
				writeEnvelope(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
