package webhook

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/store"
)

// testSecret is the fake signing key every test uses (D013).
const testSecret = "test-secret"

const syncTimeout = 10 * time.Second

func fixedNow() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestSignKnownVectors(t *testing.T) {
	cases := []struct{ secret, body, want string }{
		// RFC 4231 test case 2.
		{"Jefe", "what do ya want for nothing?", "sha256=5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"},
		// The notification body of the E2E test, computed outside Go.
		{testSecret, `{"id":"msg_1","status":"delivered"}`, "sha256=1e444f49f91acc1e9f4a1db7d1bd84536bc99e9984a1749326daf4d24ebf4ce1"},
	}
	for _, c := range cases {
		if got := Sign(c.secret, []byte(c.body)); got != c.want {
			t.Errorf("Sign(%q, %q) = %s, want %s", c.secret, c.body, got, c.want)
		}
	}
}

func TestVerify(t *testing.T) {
	body := []byte(`{"id":"msg_1","status":"delivered"}`)
	sig := Sign(testSecret, body)
	if !Verify(testSecret, body, sig) {
		t.Fatal("valid signature rejected")
	}
	for name, c := range map[string]struct {
		secret string
		body   []byte
		header string
	}{
		"tampered body": {testSecret, []byte(`{"id":"msg_1","status":"undelivered"}`), sig},
		"wrong secret":  {"other-secret", body, sig},
		"no prefix":     {testSecret, body, strings.TrimPrefix(sig, "sha256=")},
		"not hex":       {testSecret, body, "sha256=zz"},
		"empty":         {testSecret, body, ""},
	} {
		if Verify(c.secret, c.body, c.header) {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Verify must compare MACs with hmac.Equal and never with ==, so the time it
// takes does not depend on how many leading bytes match.
func TestVerifyComparesInConstantTime(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "webhook.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var verify *ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "Verify" {
			verify = fn
		}
	}
	if verify == nil {
		t.Fatal("func Verify not found")
	}
	equalCalls := 0
	ast.Inspect(verify.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "hmac" && sel.Sel.Name == "Equal" {
					equalCalls++
				}
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "bytes" && sel.Sel.Name == "Equal" {
					t.Error("Verify uses bytes.Equal")
				}
			}
		case *ast.BinaryExpr:
			if n.Op == token.EQL {
				t.Error("Verify compares with ==")
			}
		}
		return true
	})
	if equalCalls != 1 {
		t.Fatalf("Verify calls hmac.Equal %d times, want 1", equalCalls)
	}
}

// stubProvider answers ParseCallback with a scripted report or error.
type stubProvider struct {
	name string
	rep  provider.DeliveryReport
	err  error
}

func (p *stubProvider) Name() string { return p.name }
func (p *stubProvider) Send(context.Context, provider.OutboundMessage) (provider.ProviderReceipt, error) {
	return provider.ProviderReceipt{}, errors.New("stub: send not used")
}
func (p *stubProvider) ParseCallback(*http.Request) (provider.DeliveryReport, error) {
	return p.rep, p.err
}

// targets is a Targets backed by a map of tenant → url.
type targets map[string]string

func (m targets) Callback(tenantID string) (string, string) {
	if u := m[tenantID]; u != "" {
		return u, testSecret
	}
	return "", ""
}

// sentMessage stores a message for tenant and moves it to sent with ref.
func sentMessage(t *testing.T, st *store.Memory, id, tenant, ref string) {
	t.Helper()
	if _, err := st.Create(store.Message{ID: id, TenantID: tenant, To: "+886900000001", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Transition(id, store.StatusQueued, store.StatusSending, store.Patch{}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Transition(id, store.StatusSending, store.StatusSent, store.Patch{Attempts: 1, ProviderRef: ref}); err != nil {
		t.Fatal(err)
	}
}

func callbackRequest() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/v1/providers/stub/callbacks", strings.NewReader("x"))
}

func TestInboundDeliveredThenDuplicate(t *testing.T) {
	st := store.New(fixedNow)
	sentMessage(t, st, "m", "a", "ref-1")
	p := &stubProvider{name: "stub", rep: provider.DeliveryReport{ProviderRef: "ref-1", Status: provider.StatusDelivered}}
	in := NewInbound(st, targets{"a": "http://tenant.invalid/hook"}, p)

	out, err := in.Receive("stub", callbackRequest())
	if err != nil || out != Applied {
		t.Fatalf("first callback: %v %v", out, err)
	}
	if m, _ := st.GetByID("m"); m.Status != store.StatusDelivered {
		t.Fatalf("status = %s", m.Status)
	}
	for i := 0; i < 3; i++ {
		out, err = in.Receive("stub", callbackRequest())
		if err != nil || out != Duplicate {
			t.Fatalf("repeat %d: %v %v", i, out, err)
		}
	}
	if got := st.Notifications("m"); len(got) != 1 || got[0].Status != store.StatusDelivered {
		t.Fatalf("outbox = %+v", got)
	}
}

func TestInboundUndeliveredCarriesReason(t *testing.T) {
	st := store.New(fixedNow)
	sentMessage(t, st, "m", "a", "ref-1")
	in := NewInbound(st, targets{"a": "http://tenant.invalid/hook"})
	out, err := in.Apply(provider.DeliveryReport{ProviderRef: "ref-1", Status: provider.StatusUndelivered})
	if err != nil || out != Applied {
		t.Fatalf("apply: %v %v", out, err)
	}
	m, _ := st.GetByID("m")
	if m.Status != store.StatusUndelivered || m.FailureReason != ReasonUndelivered {
		t.Fatalf("message = %s %q", m.Status, m.FailureReason)
	}
	// A later, different report for a terminal message changes nothing.
	out, err = in.Apply(provider.DeliveryReport{ProviderRef: "ref-1", Status: provider.StatusDelivered})
	if err != nil || out != Duplicate {
		t.Fatalf("late delivered: %v %v", out, err)
	}
	if m, _ := st.GetByID("m"); m.Status != store.StatusUndelivered {
		t.Fatalf("terminal status changed to %s", m.Status)
	}
}

func TestInboundTenantWithoutCallbackURLIsNotEnqueued(t *testing.T) {
	st := store.New(fixedNow)
	sentMessage(t, st, "m", "b", "ref-1")
	in := NewInbound(st, targets{})
	if out, err := in.Apply(provider.DeliveryReport{ProviderRef: "ref-1", Status: provider.StatusDelivered}); err != nil || out != Applied {
		t.Fatalf("apply: %v %v", out, err)
	}
	if got := st.Notifications("m"); len(got) != 0 {
		t.Fatalf("outbox = %+v", got)
	}
}

func TestInboundErrors(t *testing.T) {
	st := store.New(fixedNow)
	sentMessage(t, st, "sent", "a", "ref-sent")
	if _, err := st.Create(store.Message{ID: "queued", TenantID: "a", To: "+886900000001", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		p    *stubProvider
		path string
		want error
	}{
		{"unknown provider", &stubProvider{name: "stub"}, "other", ErrUnknownProvider},
		{"unauthorized", &stubProvider{name: "stub", err: fmt.Errorf("bad secret: %w", provider.ErrUnauthorized)}, "stub", ErrUnauthorized},
		{"undecodable", &stubProvider{name: "stub", err: fmt.Errorf("no ref: %w", provider.ErrPermanent)}, "stub", ErrInvalidReport},
		{"status sent", &stubProvider{name: "stub", rep: provider.DeliveryReport{ProviderRef: "ref-sent", Status: provider.StatusSent}}, "stub", ErrInvalidReport},
		{"status failed", &stubProvider{name: "stub", rep: provider.DeliveryReport{ProviderRef: "ref-sent", Status: provider.StatusFailed}}, "stub", ErrInvalidReport},
		{"unknown ref", &stubProvider{name: "stub", rep: provider.DeliveryReport{ProviderRef: "nope", Status: provider.StatusDelivered}}, "stub", ErrUnknownMessage},
		{"empty ref", &stubProvider{name: "stub", rep: provider.DeliveryReport{Status: provider.StatusDelivered}}, "stub", ErrUnknownMessage},
	}
	for _, c := range cases {
		in := NewInbound(st, targets{"a": "http://tenant.invalid/hook"}, c.p)
		if out, err := in.Receive(c.path, callbackRequest()); !errors.Is(err, c.want) || out != 0 {
			t.Errorf("%s: %v %v, want %v", c.name, out, err, c.want)
		}
	}
	if m, _ := st.GetByID("sent"); m.Status != store.StatusSent {
		t.Fatalf("rejected callbacks changed status to %s", m.Status)
	}
	if got := st.Notifications("sent"); len(got) != 0 {
		t.Fatalf("rejected callbacks enqueued %+v", got)
	}
}

// A callback before sent is written cannot match a message yet: queued and
// sending messages have no ProviderRef, so it is ErrUnknownMessage (404) and
// the supplier retries.
func TestInboundBeforeSentIsUnknown(t *testing.T) {
	st := store.New(fixedNow)
	if _, err := st.Create(store.Message{ID: "m", TenantID: "a", To: "+886900000001", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	in := NewInbound(st, targets{"a": "http://tenant.invalid/hook"})
	if _, err := in.Apply(provider.DeliveryReport{ProviderRef: "ref-1", Status: provider.StatusDelivered}); !errors.Is(err, ErrUnknownMessage) {
		t.Fatalf("apply: %v", err)
	}
}

func TestInboundConcurrentDuplicatesApplyOnce(t *testing.T) {
	st := store.New(fixedNow)
	sentMessage(t, st, "m", "a", "ref-1")
	in := NewInbound(st, targets{"a": "http://tenant.invalid/hook"})
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[Outcome]int{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := in.Apply(provider.DeliveryReport{ProviderRef: "ref-1", Status: provider.StatusDelivered})
			if err != nil {
				t.Error(err)
			}
			mu.Lock()
			counts[out]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if counts[Applied] != 1 || counts[Duplicate] != 19 {
		t.Fatalf("outcomes = %v", counts)
	}
	if got := st.Notifications("m"); len(got) != 1 {
		t.Fatalf("outbox = %+v", got)
	}
}

// recorder is a Doer answering with scripted status codes (then 204) and
// recording every request.
type recorder struct {
	mu       sync.Mutex
	statuses []int
	reqs     []recorded
}

type recorded struct {
	url, sig, contentType string
	body                  []byte
}

func (r *recorder) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, recorded{req.URL.String(), req.Header.Get(SignatureHeader), req.Header.Get("Content-Type"), body})
	code := http.StatusNoContent
	if len(r.statuses) > 0 {
		code, r.statuses = r.statuses[0], r.statuses[1:]
	}
	if code == 0 {
		return nil, errors.New("recorder: connection refused")
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (r *recorder) requests() []recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recorded(nil), r.reqs...)
}

// waits records back-off durations without blocking.
type waits struct {
	mu sync.Mutex
	ds []time.Duration
}

func (w *waits) Wait(ctx context.Context, d time.Duration) error {
	w.mu.Lock()
	w.ds = append(w.ds, d)
	w.mu.Unlock()
	return ctx.Err()
}

func (w *waits) all() []time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]time.Duration(nil), w.ds...)
}

// finishedOutbox reports every finished entry on done.
type finishedOutbox struct {
	*store.Memory
	done chan store.NotificationState
}

func (o *finishedOutbox) FinishNotification(seq int64, state store.NotificationState) error {
	err := o.Memory.FinishNotification(seq, state)
	o.done <- state
	return err
}

// dispatch runs a Dispatcher over st until the test ends and returns the
// channel of finished entries.
func dispatch(t *testing.T, st *store.Memory, tg Targets, doer Doer, w *waits) <-chan store.NotificationState {
	t.Helper()
	ob := &finishedOutbox{Memory: st, done: make(chan store.NotificationState, 16)}
	d := NewDispatcher(ob, tg, doer, w.Wait)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- d.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-stopped; err != nil {
			t.Errorf("Run: %v", err)
		}
	})
	return ob.done
}

func awaitFinished(t *testing.T, done <-chan store.NotificationState) store.NotificationState {
	t.Helper()
	select {
	case s := <-done:
		return s
	case <-time.After(syncTimeout):
		t.Fatal("no notification finished")
		return ""
	}
}

// delivered moves message id (tenant a) to delivered and enqueues it.
func delivered(t *testing.T, st *store.Memory, id string) {
	t.Helper()
	sentMessage(t, st, id, "a", "ref-"+id)
	if _, err := st.TransitionAndNotify(id, store.StatusSent, store.StatusDelivered, store.Patch{}); err != nil {
		t.Fatal(err)
	}
}

func TestDispatcherPostsSignedBodyOnce(t *testing.T) {
	st := store.New(fixedNow)
	rec := &recorder{}
	done := dispatch(t, st, targets{"a": "http://tenant.invalid/hook"}, rec, &waits{})
	delivered(t, st, "msg_1")
	if s := awaitFinished(t, done); s != store.NotificationDone {
		t.Fatalf("finished %s", s)
	}
	reqs := rec.requests()
	if len(reqs) != 1 {
		t.Fatalf("%d requests", len(reqs))
	}
	r := reqs[0]
	if r.url != "http://tenant.invalid/hook" || r.contentType != "application/json" || string(r.body) != `{"id":"msg_1","status":"delivered"}` {
		t.Fatalf("request = %+v (body %s)", r, r.body)
	}
	if r.sig != "sha256=1e444f49f91acc1e9f4a1db7d1bd84536bc99e9984a1749326daf4d24ebf4ce1" || !Verify(testSecret, r.body, r.sig) {
		t.Fatalf("signature %q", r.sig)
	}
}

func TestDispatcherBacksOffThenSucceeds(t *testing.T) {
	st := store.New(fixedNow)
	rec := &recorder{statuses: []int{http.StatusInternalServerError, 0}}
	w := &waits{}
	done := dispatch(t, st, targets{"a": "http://tenant.invalid/hook"}, rec, w)
	delivered(t, st, "m")
	if s := awaitFinished(t, done); s != store.NotificationDone {
		t.Fatalf("finished %s", s)
	}
	if got := w.all(); fmt.Sprint(got) != "[1s 2s]" {
		t.Fatalf("waits = %v", got)
	}
	if n := len(rec.requests()); n != 3 {
		t.Fatalf("%d requests, want 3", n)
	}
}

func TestDispatcherExhaustedAbandonsWithoutChangingMessage(t *testing.T) {
	st := store.New(fixedNow)
	rec := &recorder{statuses: []int{500, 502, 0, 404, 500}}
	w := &waits{}
	done := dispatch(t, st, targets{"a": "http://tenant.invalid/hook"}, rec, w)
	delivered(t, st, "m")
	if s := awaitFinished(t, done); s != store.NotificationAbandoned {
		t.Fatalf("finished %s", s)
	}
	if got := w.all(); fmt.Sprint(got) != "[1s 2s 4s]" {
		t.Fatalf("waits = %v", got)
	}
	if n := len(rec.requests()); n != MaxAttempts {
		t.Fatalf("%d requests, want %d", n, MaxAttempts)
	}
	if m, _ := st.GetByID("m"); m.Status != store.StatusDelivered {
		t.Fatalf("abandoned notification changed status to %s", m.Status)
	}
	if got := st.Notifications("m"); got[0].State != store.NotificationAbandoned {
		t.Fatalf("outbox = %+v", got)
	}
}

func TestDispatcherSendsInEnqueueOrder(t *testing.T) {
	st := store.New(fixedNow)
	rec := &recorder{statuses: []int{500}}
	done := dispatch(t, st, targets{"a": "http://tenant.invalid/hook"}, rec, &waits{})
	for _, id := range []string{"m1", "m2", "m3"} {
		delivered(t, st, id)
	}
	for i := 0; i < 3; i++ {
		if s := awaitFinished(t, done); s != store.NotificationDone {
			t.Fatalf("finished %s", s)
		}
	}
	var got []string
	for _, r := range rec.requests() {
		got = append(got, string(r.body))
	}
	want := []string{
		`{"id":"m1","status":"delivered"}`, `{"id":"m1","status":"delivered"}`,
		`{"id":"m2","status":"delivered"}`, `{"id":"m3","status":"delivered"}`,
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("order = %v", got)
	}
}

func TestDispatcherWithoutCallbackURLAbandons(t *testing.T) {
	st := store.New(fixedNow)
	rec := &recorder{}
	done := dispatch(t, st, targets{}, rec, &waits{})
	delivered(t, st, "m")
	if s := awaitFinished(t, done); s != store.NotificationAbandoned {
		t.Fatalf("finished %s", s)
	}
	if n := len(rec.requests()); n != 0 {
		t.Fatalf("%d requests", n)
	}
}
