package httpapi

import (
	"time"

	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
)

// Deps are the collaborators the handler needs. Queue, Store, Tenants and
// Templates are required (NewHandler panics without them, so a bad assembly
// fails at startup). There is deliberately no Provider: only the queue talks
// to providers (AD-1).
type Deps struct {
	// Now returns the current time (injected; reserved for time-aware routes).
	Now func() time.Time
	// Queue accepts new messages.
	Queue queue.Queue
	// Store is the tenant-scoped read view.
	Store store.TenantScoped
	// Tenants resolves X-API-Key to a tenant.
	Tenants Authenticator
	// Templates is tenant-scoped template CRUD; *template.Service satisfies it.
	Templates Templates
	// Limiter rate-limits /v1 requests per API key; *ratelimit.Limiter
	// satisfies it. nil turns limiting off: cmd/smsd and e2e/testutil always
	// set it (NewHandler's required checks live in handler.go, outside T0005).
	Limiter Limiter
}
