package tenant

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
)

// ErrUnauthorized is returned for an empty, unknown or disabled API key. The
// three cases are deliberately indistinguishable to callers.
var ErrUnauthorized = errors.New("tenant: unauthorized")

// Tenant is one customer of the platform.
type Tenant struct {
	ID string
	// CallbackURL is where outbound status webhooks are sent (stored only;
	// not used until the webhook stories).
	CallbackURL string
	// RateLimitPerMin is the per-minute request limit for each of the tenant's
	// API keys; <= 0 means unset, and ratelimit applies its default.
	RateLimitPerMin int
}

// HashKey returns the SHA-256 hex digest of an API key. Only digests are ever
// stored; the plaintext key must not be logged or returned.
func HashKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

type keyRecord struct {
	hash     string
	tenantID string
	disabled bool
}

// Registry is the in-memory tenant and API-key directory.
type Registry struct {
	mu      sync.RWMutex
	tenants map[string]Tenant
	keys    []keyRecord
}

// NewRegistry returns an empty registry: every lookup is ErrUnauthorized.
func NewRegistry() *Registry {
	return &Registry{tenants: map[string]Tenant{}}
}

// AddTenant registers t, replacing any tenant with the same ID.
func (r *Registry) AddTenant(t Tenant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[t.ID] = t
}

// RateLimitPerMin returns the per-minute limit of tenantID, or 0 (unset) for
// an unknown tenant. Its signature matches what ratelimit.New takes.
func (r *Registry) RateLimitPerMin(tenantID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tenants[tenantID].RateLimitPerMin
}

// AddKey issues apiKey (stored as its hash) to an existing tenant. It reports
// false when the tenant is unknown.
func (r *Registry) AddKey(tenantID, apiKey string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[tenantID]; !ok {
		return false
	}
	r.keys = append(r.keys, keyRecord{hash: HashKey(apiKey), tenantID: tenantID})
	return true
}

// DisableKey marks apiKey as disabled. It reports false when no such key exists.
func (r *Registry) DisableKey(apiKey string) bool {
	h := []byte(HashKey(apiKey))
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.keys {
		if subtle.ConstantTimeCompare([]byte(r.keys[i].hash), h) == 1 {
			r.keys[i].disabled = true
			return true
		}
	}
	return false
}

// Authenticate resolves apiKey to its tenant. Empty, unknown and disabled keys
// all return ErrUnauthorized. Hashes are compared in constant time.
func (r *Registry) Authenticate(apiKey string) (Tenant, error) {
	if apiKey == "" {
		return Tenant{}, ErrUnauthorized
	}
	h := []byte(HashKey(apiKey))
	r.mu.RLock()
	defer r.mu.RUnlock()
	found := -1
	for i := range r.keys {
		if subtle.ConstantTimeCompare([]byte(r.keys[i].hash), h) == 1 {
			found = i
		}
	}
	if found < 0 || r.keys[found].disabled {
		return Tenant{}, ErrUnauthorized
	}
	t, ok := r.tenants[r.keys[found].tenantID]
	if !ok {
		return Tenant{}, ErrUnauthorized
	}
	return t, nil
}
