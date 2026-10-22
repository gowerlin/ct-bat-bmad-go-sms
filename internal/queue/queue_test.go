package queue

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/store"
)

func now() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

// scriptedProvider answers Send from errs in order (success once they run
// out). With a gate, every Send first reports on called and then blocks until
// the gate is closed or ctx ends.
type scriptedProvider struct {
	mu     sync.Mutex
	errs   []error
	calls  int
	gate   chan struct{}
	called chan struct{}
}

func (p *scriptedProvider) Name() string { return "scripted" }

func (p *scriptedProvider) ParseCallback(*http.Request) (provider.DeliveryReport, error) {
	return provider.DeliveryReport{}, errors.New("not used")
}

func (p *scriptedProvider) Send(ctx context.Context, _ provider.OutboundMessage) (provider.ProviderReceipt, error) {
	p.mu.Lock()
	p.calls++
	n := p.calls
	var err error
	if len(p.errs) > 0 {
		err, p.errs = p.errs[0], p.errs[1:]
	}
	p.mu.Unlock()
	if p.gate != nil {
		p.called <- struct{}{}
		select {
		case <-p.gate:
		case <-ctx.Done():
			return provider.ProviderReceipt{}, ctx.Err()
		}
	}
	if err != nil {
		return provider.ProviderReceipt{}, err
	}
	return provider.ProviderReceipt{ProviderRef: fmt.Sprintf("ref-%d", n)}, nil
}

func (p *scriptedProvider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func gated(workers int) *scriptedProvider {
	return &scriptedProvider{gate: make(chan struct{}), called: make(chan struct{}, workers)}
}

// finishedStore reports every message leaving sending on finished.
type finishedStore struct {
	store.System
	finished chan store.Message
}

func (s *finishedStore) Transition(id string, from, to store.Status, p store.Patch) (store.Message, error) {
	m, err := s.System.Transition(id, from, to, p)
	if err == nil && from == store.StatusSending {
		s.finished <- m
	}
	return m, err
}

// recordedWaits records back-off durations without blocking.
type recordedWaits struct {
	mu sync.Mutex
	ds []time.Duration
}

func (w *recordedWaits) Wait(ctx context.Context, d time.Duration) error {
	w.mu.Lock()
	w.ds = append(w.ds, d)
	w.mu.Unlock()
	return ctx.Err()
}

func (w *recordedWaits) Got() []time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]time.Duration(nil), w.ds...)
}

// running starts q.Run and stops it when the test ends; it fails the test if
// Run does not return after Close.
func running(t *testing.T, q Queue) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := q.Close(context.Background()); err != nil {
			t.Errorf("Close: %v", err)
		}
		if err := <-done; err != nil {
			t.Errorf("Run: %v", err)
		}
	})
}

func newFinishedStore() *finishedStore {
	return &finishedStore{System: store.New(now), finished: make(chan store.Message, 16)}
}

func TestSubmitStoresQueuedMessage(t *testing.T) {
	st := store.New(now)
	q := NewInProcess(st, WithProvider(&scriptedProvider{}))
	id, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"})
	if err != nil || !strings.HasPrefix(id, "msg_") {
		t.Fatalf("id=%q err=%v", id, err)
	}
	m, err := st.Get("default", id)
	if err != nil || m.Status != store.StatusQueued {
		t.Fatalf("msg=%+v err=%v", m, err)
	}
}

func TestSubmitFullLeavesNoMessage(t *testing.T) {
	st := store.New(now)
	q := newInProcess(st, 2, WithProvider(&scriptedProvider{}))
	for i := 0; i < 2; i++ {
		if _, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "x"}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("err=%v", err)
	}
	if st.Len() != 2 {
		t.Fatalf("store has %d messages, want 2", st.Len())
	}
}

func TestSubmitStoreFailureReleasesReservation(t *testing.T) {
	st := store.New(now)
	q := newInProcess(st, 1, WithProvider(&scriptedProvider{}))
	// an empty tenant makes Create fail
	if _, err := q.Submit(context.Background(), "", Draft{}); err == nil {
		t.Fatal("want error")
	}
	if _, err := q.Submit(context.Background(), "default", Draft{}); err != nil {
		t.Fatalf("slot not released: %v", err)
	}
}

func TestCloseRejectsSubmit(t *testing.T) {
	q := NewInProcess(store.New(now), WithProvider(&scriptedProvider{}))
	q.Close(context.Background())
	if _, err := q.Submit(context.Background(), "default", Draft{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("err=%v", err)
	}
}

func TestDeliverOutcomes(t *testing.T) {
	transient := fmt.Errorf("x: %w", provider.ErrTransient)
	cases := []struct {
		name      string
		errs      []error
		status    store.Status
		attempts  int
		reason    string
		ref       string
		wantWaits []time.Duration
	}{
		{"first try succeeds", nil, store.StatusSent, 1, "", "ref-1", nil},
		{"retries then succeeds", []error{transient, transient}, store.StatusSent, 3, "", "ref-3",
			[]time.Duration{time.Second, 2 * time.Second}},
		{"retries exhausted", []error{transient, transient, transient, transient, transient}, store.StatusFailed, 4,
			ReasonRetriesExhausted, "", []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}},
		{"permanent is not retried", []error{fmt.Errorf("x: %w", provider.ErrPermanent)}, store.StatusFailed, 1,
			ReasonPermanent, "", nil},
		{"unclassified counts as permanent", []error{errors.New("boom")}, store.StatusFailed, 1,
			ReasonPermanent, "", nil},
		{"permanent after a retry", []error{transient, provider.ErrPermanent}, store.StatusFailed, 2,
			ReasonPermanent, "", []time.Duration{time.Second}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := newFinishedStore()
			prov := &scriptedProvider{errs: c.errs}
			waits := &recordedWaits{}
			q := NewInProcess(st, WithProvider(prov), WithWait(waits.Wait))
			running(t, q)

			id, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"})
			if err != nil {
				t.Fatal(err)
			}
			m := <-st.finished
			if m.ID != id || m.Status != c.status || m.Attempts != c.attempts ||
				m.FailureReason != c.reason || m.ProviderRef != c.ref {
				t.Fatalf("finished %+v, want status=%s attempts=%d reason=%q ref=%q",
					m, c.status, c.attempts, c.reason, c.ref)
			}
			if prov.Calls() != c.attempts {
				t.Fatalf("provider called %d times, want %d", prov.Calls(), c.attempts)
			}
			if got := waits.Got(); fmt.Sprint(got) != fmt.Sprint(c.wantWaits) {
				t.Fatalf("waits = %v, want %v", got, c.wantWaits)
			}
		})
	}
}

func TestRunsDefaultWorkersConcurrently(t *testing.T) {
	st := newFinishedStore()
	prov := gated(DefaultWorkers)
	q := NewInProcess(st, WithProvider(prov))
	running(t, q)

	for i := 0; i < DefaultWorkers+1; i++ {
		if _, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"}); err != nil {
			t.Fatal(err)
		}
	}
	// DefaultWorkers sends are in flight at once while the gate is shut.
	for i := 0; i < DefaultWorkers; i++ {
		<-prov.called
	}
	close(prov.gate)
	<-prov.called // the extra message gets a free worker
	for i := 0; i < DefaultWorkers+1; i++ {
		if m := <-st.finished; m.Status != store.StatusSent {
			t.Fatalf("finished %+v", m)
		}
	}
}

func TestDequeueReleasesSlot(t *testing.T) {
	st := newFinishedStore()
	q := newInProcess(st, 1, WithProvider(&scriptedProvider{}))
	running(t, q)
	for i := 0; i < 3; i++ {
		if _, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"}); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		<-st.finished
	}
}

func TestCloseWaitsForInFlightSend(t *testing.T) {
	st := newFinishedStore()
	prov := gated(1)
	q := NewInProcess(st, WithProvider(prov), WithWorkers(1))
	runDone := make(chan error, 1)
	go func() { runDone <- q.Run(context.Background()) }()

	id, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	<-prov.called
	closed := make(chan error, 1)
	go func() { closed <- q.Close(context.Background()) }()
	// Open the gate only once Close has begun, so a Close that did not wait
	// would return while the send is still blocked.
	for !q.(*inProcess).isClosed() {
		runtime.Gosched()
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned %v while a send was in flight", err)
	default:
	}
	close(prov.gate)
	if err := <-closed; err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Close returned only after the in-flight send was written.
	if m, _ := st.GetByID(id); m.Status != store.StatusSent {
		t.Fatalf("after Close status = %s, want sent", m.Status)
	}
	if err := <-runDone; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := q.Submit(context.Background(), "default", Draft{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Submit after Close: %v", err)
	}
}

func TestCloseDuringSendLeavesSending(t *testing.T) {
	st := newFinishedStore()
	prov := gated(1)
	q := NewInProcess(st, WithProvider(prov), WithWorkers(1))
	runDone := make(chan error, 1)
	go func() { runDone <- q.Run(context.Background()) }()

	id, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	<-prov.called
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.Close(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close = %v, want context.Canceled", err)
	}
	if err := <-runDone; err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The cancelled Send is neither a permanent failure nor a retry.
	if m, _ := st.GetByID(id); m.Status != store.StatusSending || m.FailureReason != "" || prov.Calls() != 1 {
		t.Fatalf("aborted message: %+v calls=%d, want sending, no reason, 1 call", m, prov.Calls())
	}
}

func TestRunCtxEndFinishesInFlightSend(t *testing.T) {
	st := newFinishedStore()
	prov := gated(1)
	q := NewInProcess(st, WithProvider(prov), WithWorkers(1))
	ctx, stopRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- q.Run(ctx) }()

	id, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	<-prov.called
	stopRun()
	// Run's ctx ending refuses new messages like Close does ...
	for !q.(*inProcess).isClosed() {
		runtime.Gosched()
	}
	if _, err := q.Submit(context.Background(), "default", Draft{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Submit after Run's ctx ended = %v, want ErrClosed", err)
	}
	// ... but the send already in flight still completes.
	close(prov.gate)
	select {
	case m := <-st.finished:
		if m.ID != id || m.Status != store.StatusSent {
			t.Fatalf("finished %+v, want %s sent", m, id)
		}
	case err := <-runDone:
		t.Fatalf("Run returned %v before the in-flight send finished", err)
	}
	if err := <-runDone; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := q.Run(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Run again = %v, want ErrClosed", err)
	}
}

func TestCloseCancelsInFlightWhenCtxEnds(t *testing.T) {
	st := newFinishedStore()
	waiting := make(chan struct{}, 1)
	wait := func(ctx context.Context, _ time.Duration) error {
		waiting <- struct{}{}
		<-ctx.Done() // a back-off that never ends on its own
		return ctx.Err()
	}
	prov := &scriptedProvider{errs: []error{provider.ErrTransient}}
	q := NewInProcess(st, WithProvider(prov), WithWait(wait), WithWorkers(1))
	runCtx, stopRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- q.Run(runCtx) }()

	id, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	<-waiting
	// Ending Run's ctx stops new work but does not cancel the back-off.
	stopRun()
	select {
	case err := <-runDone:
		t.Fatalf("Run returned %v while a message was in flight", err)
	default:
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.Close(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close = %v, want context.Canceled", err)
	}
	if err := <-runDone; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m, _ := st.GetByID(id); m.Status != store.StatusSending || prov.Calls() != 1 {
		t.Fatalf("aborted message: status=%s calls=%d, want sending/1", m.Status, prov.Calls())
	}
}

func TestRunReturnsWhenCtxEnds(t *testing.T) {
	q := NewInProcess(store.New(now), WithProvider(&scriptedProvider{}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.Run(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := q.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestRunRefusals(t *testing.T) {
	st := newFinishedStore()
	q := NewInProcess(st, WithProvider(&scriptedProvider{}))
	running(t, q)
	// A delivered message proves the first Run has started its workers.
	if _, err := q.Submit(context.Background(), "default", Draft{To: "+886900000001", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	<-st.finished
	if err := q.Run(context.Background()); !errors.Is(err, ErrRunning) {
		t.Fatalf("second Run = %v, want ErrRunning", err)
	}

	closed := NewInProcess(store.New(now), WithProvider(&scriptedProvider{}))
	closed.Close(context.Background())
	if err := closed.Run(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Run after Close = %v, want ErrClosed", err)
	}

	if err := NewInProcess(store.New(now)).Run(context.Background()); !errors.Is(err, ErrNoProvider) {
		t.Fatalf("Run without a provider = %v, want ErrNoProvider", err)
	}
}
