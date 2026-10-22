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
)

// Env is one running test server plus the fakes wired into it.
type Env struct {
	Server   *httptest.Server
	Client   *http.Client
	Clock    *FakeClock
	Provider *FakeProvider
}

// NewTestServer starts the platform handler (the same one cmd/smsd serves)
// on a loopback httptest.Server and closes it when the test ends.
func NewTestServer(t testing.TB) *Env {
	t.Helper()
	clock := NewFakeClock()
	prov := NewFakeProvider(clock.Now)
	h := httpapi.NewHandler(httpapi.Deps{Now: clock.Now, Provider: prov})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Env{Server: srv, Client: srv.Client(), Clock: clock, Provider: prov}
}

// URL joins the server base URL with path.
func (e *Env) URL(path string) string {
	return e.Server.URL + "/" + strings.TrimPrefix(path, "/")
}

// Get performs a GET and returns the response with its body fully read.
func (e *Env) Get(t testing.TB, path string) (*http.Response, []byte) {
	t.Helper()
	return e.Do(t, http.MethodGet, path, "", nil)
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
