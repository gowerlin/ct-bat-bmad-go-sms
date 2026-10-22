// Package testutil is the E2E harness: an httptest server wrapping the real
// httpapi handler, with a FakeClock and a FakeProvider injected so every
// TestE2E_* case is deterministic and needs no network.
package testutil

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/sms-platform/internal/httpapi"
	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/ratelimit"
	"example.com/sms-platform/internal/store"
	"example.com/sms-platform/internal/template"
	"example.com/sms-platform/internal/tenant"
	"example.com/sms-platform/internal/webhook"
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

	watch  *watchedStore
	outbox *watchedOutbox
}

// Option adjusts NewTestServer.
type Option func(*options)

type options struct {
	noWorker bool
}

// WithoutWorker leaves the queue's send workers stopped, so every submitted
// message stays queued unless the test moves it through Env.Store itself.
func WithoutWorker() Option {
	return func(o *options) { o.noWorker = true }
}

// NewTestServer starts the platform handler (the same one cmd/smsd serves)
// on a loopback httptest.Server and closes it when the test ends. Unless
// WithoutWorker is given, the queue's send workers run against the
// FakeProvider and back off on FakeClock.Wait; they are stopped when the test
// ends. Supplier callbacks for the FakeProvider go to webhook.Inbound, and
// the webhook Dispatcher always runs, retrying on FakeClock.Wait; tenants get
// notifications once a test gives them an endpoint with NewReceiver.
func NewTestServer(t testing.TB, opts ...Option) *Env {
	t.Helper()
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	clock := NewFakeClock()
	prov := NewFakeProvider(clock.Now)
	st := store.New(clock.Now)
	watch := &watchedStore{System: st, changed: make(chan struct{})}
	q := queue.NewInProcess(watch, queue.WithProvider(prov), queue.WithWait(clock.Wait))
	reg := tenant.NewRegistry()
	for id, key := range map[string]string{TenantA: KeyA, TenantB: KeyB} {
		reg.AddTenant(tenant.Tenant{ID: id, RateLimitPerMin: TestRateLimitPerMin})
		reg.AddKey(id, key)
	}
	h := httpapi.NewHandler(httpapi.Deps{
		Now: clock.Now, Queue: q, Store: st, Tenants: reg, Templates: template.NewService(st),
		Limiter:   ratelimit.New(clock.Now, reg.RateLimitPerMin),
		Callbacks: webhook.NewInbound(st, reg, prov),
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	outbox := &watchedOutbox{Memory: st, changed: make(chan struct{})}
	startDispatcher(t, webhook.NewDispatcher(outbox, reg, newReceiverClient(t), clock.Wait))
	if !o.noWorker {
		startWorkers(t, q)
	}
	return &Env{
		Server: srv, Client: srv.Client(), Clock: clock, Provider: prov, Store: st, Queue: q, Tenants: reg,
		watch: watch, outbox: outbox,
	}
}

// newReceiverClient is the Dispatcher's client: plain loopback HTTP to the
// test's Receivers, nothing else is ever addressed (cmd/smsd adds the AD-9
// allow-list).
func newReceiverClient(t testing.TB) *http.Client {
	c := new(http.Client)
	c.Transport = http.DefaultTransport.(*http.Transport).Clone()
	c.Timeout = syncTimeout
	t.Cleanup(c.CloseIdleConnections)
	return c
}

// startDispatcher runs d until the test ends; cleanup cancels it and waits
// for Run to return.
func startDispatcher(t testing.TB, d *webhook.Dispatcher) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("webhook Dispatcher Run: %v", err)
		}
	})
}

// Notification is one webhook request a Receiver got.
type Notification struct {
	Body        []byte
	Signature   string // the X-Signature header
	ContentType string
}

// Receiver is a tenant's webhook endpoint: a loopback httptest.Server that
// records every notification and answers 204.
type Receiver struct {
	Server *httptest.Server
	mu     sync.Mutex
	got    []Notification
}

// NewReceiver starts a Receiver and makes it tenantID's CallbackURL, signed
// with secret (use a fake one such as "test-secret"). It is closed when the
// test ends.
func (e *Env) NewReceiver(t testing.TB, tenantID, secret string) *Receiver {
	t.Helper()
	r := &Receiver{}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.mu.Lock()
		r.got = append(r.got, Notification{
			Body: body, Signature: req.Header.Get(webhook.SignatureHeader), ContentType: req.Header.Get("Content-Type"),
		})
		r.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(r.Server.Close)
	if !e.Tenants.SetCallback(tenantID, r.Server.URL, secret) {
		t.Fatalf("NewReceiver: unknown tenant %q", tenantID)
	}
	return r
}

// Notifications returns a copy of every notification received, in order.
func (r *Receiver) Notifications() []Notification {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Notification(nil), r.got...)
}

// AwaitOutboxIdle blocks until the Dispatcher has finished (done or
// abandoned) every outbox entry. A callback enqueues before it is answered,
// so after a callback's response this is a deterministic point to count what
// the Receivers got.
func (e *Env) AwaitOutboxIdle(t testing.TB) {
	t.Helper()
	timeout := time.NewTimer(syncTimeout)
	defer timeout.Stop()
	for {
		changed := e.outbox.next()
		if e.Store.OutboxIdle() {
			return
		}
		select {
		case <-changed:
		case <-timeout.C:
			t.Fatalf("webhook outbox not idle after %v", syncTimeout)
		}
	}
}

// watchedOutbox is the webhook.Outbox handed to the Dispatcher; it signals
// every finished entry so AwaitOutboxIdle need not poll.
type watchedOutbox struct {
	*store.Memory
	mu      sync.Mutex
	changed chan struct{} // closed and replaced after each finish
}

func (o *watchedOutbox) FinishNotification(seq int64, state store.NotificationState) error {
	err := o.Memory.FinishNotification(seq, state)
	o.mu.Lock()
	close(o.changed)
	o.changed = make(chan struct{})
	o.mu.Unlock()
	return err
}

// next returns a channel closed by the next finished entry.
func (o *watchedOutbox) next() <-chan struct{} {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.changed
}

// startWorkers runs q until the test ends. Cleanup closes the queue with an
// already cancelled context, so a send still waiting on the FakeClock is
// abandoned instead of holding the test, and then waits for Run to return.
// Run reports ErrClosed when a test closed the queue before Run started.
func startWorkers(t testing.TB, q queue.Queue) {
	done := make(chan error, 1)
	go func() { done <- q.Run(context.Background()) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_ = q.Close(ctx)
		if err := <-done; err != nil && !errors.Is(err, queue.ErrClosed) {
			t.Errorf("queue Run: %v", err)
		}
	})
}

// AwaitStatus blocks until the send worker has moved message id to want and
// returns the stored message. It fails the test if the message reaches another
// terminal status first or nothing changes within syncTimeout. It only sees
// transitions made by the queue, not ones a test makes through Env.Store.
func (e *Env) AwaitStatus(t testing.TB, id string, want store.Status) store.Message {
	t.Helper()
	timeout := time.NewTimer(syncTimeout)
	defer timeout.Stop()
	for {
		changed := e.watch.next()
		m, err := e.Store.GetByID(id)
		if err != nil {
			t.Fatalf("await %s: %v", id, err)
		}
		if m.Status == want {
			return m
		}
		if store.IsTerminal(m.Status) {
			t.Fatalf("message %s ended %s (attempts %d, failure_reason %q), want %s",
				id, m.Status, m.Attempts, m.FailureReason, want)
		}
		select {
		case <-changed:
		case <-timeout.C:
			t.Fatalf("message %s still %s after %v, want %s", id, m.Status, syncTimeout, want)
		}
	}
}

// watchedStore is the store.System handed to the queue; it signals every
// successful transition so AwaitStatus need not poll.
type watchedStore struct {
	store.System
	mu      sync.Mutex
	changed chan struct{} // closed and replaced after each transition
}

func (s *watchedStore) Transition(id string, from, to store.Status, p store.Patch) (store.Message, error) {
	m, err := s.System.Transition(id, from, to, p)
	if err == nil {
		s.mu.Lock()
		close(s.changed)
		s.changed = make(chan struct{})
		s.mu.Unlock()
	}
	return m, err
}

// next returns a channel closed by the next successful transition.
func (s *watchedStore) next() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.changed
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
