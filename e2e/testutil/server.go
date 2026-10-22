// Package testutil is the E2E harness: an httptest server wrapping the real
// httpapi handler, with a FakeClock and a FakeProvider injected so every
// TestE2E_* case is deterministic and needs no network.
package testutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/sms-platform/internal/httpapi"
	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/ratelimit"
	"example.com/sms-platform/internal/store"
	"example.com/sms-platform/internal/template"
	"example.com/sms-platform/internal/tenant"
)

// Fake API keys and tenant IDs registered by NewTestServer.
const (
	TenantA = "tenant-a"
	TenantB = "tenant-b"
	KeyA    = "test-key-tenant-a"
	KeyB    = "test-key-tenant-b"
)

// TestRateLimitPerMin is the limit NewTestServer gives TenantA and TenantB. It
// is far above anything a test sends (the queue-full test sends
// queue.Capacity+1 requests on one key without advancing the clock), so only
// rate-limit tests, which lower a tenant's limit via Env.Tenants, see 429.
const TestRateLimitPerMin = 1_000_000

// Env is one running test server plus the fakes wired into it.
type Env struct {
	Server   *httptest.Server
	Client   *http.Client
	Clock    *FakeClock
	Provider *FakeProvider
	Store    *store.Memory
	Queue    queue.Queue
	// Tenants is the registry behind the handler; tests may add or disable keys.
	Tenants *tenant.Registry
}

// NewTestServer starts the platform handler (the same one cmd/smsd serves)
// on a loopback httptest.Server and closes it when the test ends.
func NewTestServer(t testing.TB) *Env {
	t.Helper()
	clock := NewFakeClock()
	prov := NewFakeProvider(clock.Now)
	st := store.New(clock.Now)
	q := queue.NewInProcess(st)
	reg := tenant.NewRegistry()
	for id, key := range map[string]string{TenantA: KeyA, TenantB: KeyB} {
		reg.AddTenant(tenant.Tenant{ID: id, RateLimitPerMin: TestRateLimitPerMin})
		reg.AddKey(id, key)
	}
	h := httpapi.NewHandler(httpapi.Deps{
		Now: clock.Now, Queue: q, Store: st, Tenants: reg, Templates: template.NewService(st),
		Limiter: ratelimit.New(clock.Now, reg.RateLimitPerMin),
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Env{Server: srv, Client: srv.Client(), Clock: clock, Provider: prov, Store: st, Queue: q, Tenants: reg}
}

// URL joins the server base URL with path.
func (e *Env) URL(path string) string {
	return e.Server.URL + "/" + strings.TrimPrefix(path, "/")
}

// Get performs a GET as tenant A (KeyA) and returns the response with its body
// fully read.
func (e *Env) Get(t testing.TB, path string) (*http.Response, []byte) {
	t.Helper()
	return e.Do(t, http.MethodGet, path, KeyA, nil)
}

// Do performs a request with an optional JSON body and reads the response.
func (e *Env) Do(t testing.TB, method, path, apiKey string, body io.Reader) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, e.URL(path), body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := e.Client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, data
}
