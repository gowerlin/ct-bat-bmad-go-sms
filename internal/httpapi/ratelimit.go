package httpapi

import (
	"net/http"

	"example.com/sms-platform/internal/tenant"
)

// Limiter takes one request from an API key's bucket; *ratelimit.Limiter
// satisfies it. A rejection is a *ratelimit.LimitedError (429, rateLimitError).
type Limiter interface {
	Allow(tenantID, key string) error
}

// rateLimit sits between authenticate and the handler (v1), so only valid keys
// reach it and every request here has a tenant. Buckets are keyed by the API
// key's hash, so the plaintext key is never kept.
func (s *server) rateLimit(next http.Handler) http.Handler {
	if s.deps.Limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := tenant.HashKey(r.Header.Get(apiKeyHeader))
		if err := s.deps.Limiter.Allow(tenantID(r.Context()), key); err != nil {
			writeError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}
