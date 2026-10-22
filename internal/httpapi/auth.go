package httpapi

import (
	"context"
	"net/http"

	"example.com/sms-platform/internal/tenant"
)

// apiKeyHeader is the only place an API key is read from. A key in the query
// string is ignored on purpose (D002), so it can't leak into access logs.
const apiKeyHeader = "X-API-Key"

// Authenticator resolves an API key to its tenant; *tenant.Registry satisfies it.
type Authenticator interface {
	Authenticate(apiKey string) (tenant.Tenant, error)
}

type tenantKey struct{}

// tenantID reads the tenant placed in the request context by authenticate. It
// is the only source of a request's tenant.
func tenantID(ctx context.Context) string {
	id, _ := ctx.Value(tenantKey{}).(string)
	return id
}

// authenticate rejects requests without a valid, enabled X-API-Key (401) and
// otherwise stores the resolved tenant ID in the request context.
func (s *server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, err := s.deps.Tenants.Authenticate(r.Header.Get(apiKeyHeader))
		if err != nil {
			writeError(w, errUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), tenantKey{}, t.ID)))
	})
}
