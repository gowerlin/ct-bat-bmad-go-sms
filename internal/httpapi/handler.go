package httpapi

import (
	"context"
	"net/http"
	"time"

	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
)

// DefaultTenantID is the fixed tenant used until API keys arrive in Epic 2.
const DefaultTenantID = "default"

// Deps are the collaborators the handler needs. Queue and Store are required
// (NewHandler panics without them, so a bad assembly fails at startup). There
// is deliberately no Provider: only the queue talks to providers (AD-1).
type Deps struct {
	// Now returns the current time (injected; reserved for time-aware routes).
	Now func() time.Time
	// Queue accepts new messages.
	Queue queue.Queue
	// Store is the tenant-scoped read view.
	Store store.TenantScoped
}

type tenantKey struct{}

// tenantID reads the tenant placed in the request context by withTenant.
func tenantID(ctx context.Context) string {
	id, _ := ctx.Value(tenantKey{}).(string)
	return id
}

// withTenant stands in for the Epic 2 authentication middleware.
func withTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), tenantKey{}, DefaultTenantID)))
	})
}

// NewHandler builds the platform's HTTP handler. The route table mirrors
// docs/openapi.yaml.
func NewHandler(deps Deps) http.Handler {
	if deps.Queue == nil || deps.Store == nil {
		panic("httpapi: Deps.Queue and Deps.Store are required")
	}
	s := &server{deps: deps}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.Handle("POST /v1/messages", withTenant(s.handler(s.sendMessage)))
	mux.Handle("GET /v1/messages/{id}", withTenant(s.handler(s.getMessage)))
	return envelopeFallbacks(mux)
}

type server struct{ deps Deps }

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
