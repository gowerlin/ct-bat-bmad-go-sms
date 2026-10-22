package httpapi

import (
	"net/http"
	"time"

	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
)

// Deps are the collaborators the handler needs. Queue, Store and Tenants are required
// (NewHandler panics without them, so a bad assembly fails at startup). There
// is deliberately no Provider: only the queue talks to providers (AD-1).
type Deps struct {
	// Now returns the current time (injected; reserved for time-aware routes).
	Now func() time.Time
	// Queue accepts new messages.
	Queue queue.Queue
	// Store is the tenant-scoped read view.
	Store store.TenantScoped
	// Tenants resolves X-API-Key to a tenant.
	Tenants Authenticator
}

// NewHandler builds the platform's HTTP handler. The route table mirrors
// docs/openapi.yaml.
func NewHandler(deps Deps) http.Handler {
	if deps.Queue == nil || deps.Store == nil || deps.Tenants == nil {
		panic("httpapi: Deps.Queue, Deps.Store and Deps.Tenants are required")
	}
	s := &server{deps: deps}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.Handle("POST /v1/messages", s.authenticate(s.handler(s.sendMessage)))
	mux.Handle("GET /v1/messages/{id}", s.authenticate(s.handler(s.getMessage)))
	return envelopeFallbacks(mux)
}

type server struct{ deps Deps }

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
