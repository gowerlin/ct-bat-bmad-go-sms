package queue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/store"
)

// Capacity is the bounded queue size (AD-14).
const Capacity = 1000

// DefaultWorkers is the number of send workers unless WithWorkers says otherwise (AD-2).
const DefaultWorkers = 4

// MaxAttempts is how many times one message is handed to the Provider: the
// first try plus len(backoff) retries (AD-13).
const MaxAttempts = 4

// backoff is the wait before each retry, in order (AD-13).
var backoff = [MaxAttempts - 1]time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

// Failure reasons written with the sending → failed transition (AD-13).
const (
	ReasonRetriesExhausted = "重試用盡"
	ReasonPermanent        = "永久失敗"
)

var (
	// ErrQueueFull is returned by Submit when no slot is free; no message is kept.
	ErrQueueFull = errors.New("queue: full")
	// ErrClosed is returned by Submit after Close, and by Run after Close.
	ErrClosed = errors.New("queue: closed")
	// ErrRunning is returned by a second call to Run.
	ErrRunning = errors.New("queue: already running")
	// ErrNoProvider is returned by Run on a queue built without WithProvider.
	ErrNoProvider = errors.New("queue: no provider")
)

// Draft is what a caller asks to send; the queue owns id and status.
type Draft struct {
	To   string
	Body string
}

// Queue is the only queue surface callers see (AD-2).
type Queue interface {
	// Submit reserves a slot, writes the message to the store as queued and
	// enqueues it. Any failure releases the reservation and leaves no message.
	Submit(ctx context.Context, tenantID string, d Draft) (string, error)
	// Run starts the send workers and blocks until ctx ends or Close is
	// called, then waits for the workers to finish their current message.
	// When ctx ends the queue stops accepting messages, as after Close, but
	// in-flight sends are not cancelled; Run can be called only once.
	Run(ctx context.Context) error
	// Close stops accepting messages and waits for in-flight sends; unsent
	// ones are dropped (AD-14). When ctx ends first, in-flight sends are
	// cancelled, Close still waits for the workers to return and reports
	// ctx.Err().
	Close(ctx context.Context) error
}

// Option configures NewInProcess (AD-2, AD-10).
type Option func(*inProcess)

// WithProvider sets the Provider that receives every message. Run needs one;
// a queue built without it can only accept messages.
func WithProvider(p provider.Provider) Option { return func(q *inProcess) { q.prov = p } }

// WithWait replaces the back-off wait; wait must block for d or until ctx
// ends. The default waits on a real timer; nil keeps it.
func WithWait(wait func(ctx context.Context, d time.Duration) error) Option {
	return func(q *inProcess) {
		if wait != nil {
			q.wait = wait
		}
	}
}

// WithWorkers sets the number of send workers; n <= 0 keeps DefaultWorkers.
func WithWorkers(n int) Option {
	return func(q *inProcess) {
		if n > 0 {
			q.workers = n
		}
	}
}

type inProcess struct {
	store   store.System
	prov    provider.Provider
	wait    func(context.Context, time.Duration) error
	workers int
	cap     int

	// sendCtx is the context of every Provider call and back-off wait. Only
	// Close cancels it (abort), so a message already being sent is finished
	// even after Run's ctx ends.
	sendCtx context.Context
	abort   context.CancelFunc

	mu       sync.Mutex
	reserved int
	closed   bool
	running  bool
	ch       chan string
	quit     chan struct{} // closed once workers must stop taking messages
	quitOnce sync.Once
	wg       sync.WaitGroup
}

// NewInProcess returns the in-process queue with Capacity slots.
func NewInProcess(st store.System, opts ...Option) Queue { return newInProcess(st, Capacity, opts...) }

func newInProcess(st store.System, capacity int, opts ...Option) *inProcess {
	sendCtx, abort := context.WithCancel(context.Background())
	q := &inProcess{
		store: st, wait: timerWait, workers: DefaultWorkers, cap: capacity,
		sendCtx: sendCtx, abort: abort,
		ch: make(chan string, capacity), quit: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

// timerWait is the production Wait: a real timer that honours ctx.
func timerWait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *inProcess) reserve() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	if q.reserved >= q.cap {
		return ErrQueueFull
	}
	q.reserved++
	return nil
}

func (q *inProcess) release() {
	q.mu.Lock()
	q.reserved--
	q.mu.Unlock()
}

func (q *inProcess) Submit(ctx context.Context, tenantID string, d Draft) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := q.reserve(); err != nil {
		return "", err
	}
	id, err := store.NewID("msg_")
	if err != nil {
		q.release()
		return "", err
	}
	if _, err := q.store.Create(store.Message{ID: id, TenantID: tenantID, To: d.To, Body: d.Body}); err != nil {
		q.release()
		return "", err
	}
	// The reservation guarantees room in the channel, so this never blocks.
	q.ch <- id
	return id, nil
}

func (q *inProcess) Run(ctx context.Context) error {
	q.mu.Lock()
	switch {
	case q.prov == nil:
		q.mu.Unlock()
		return ErrNoProvider
	case q.closed:
		q.mu.Unlock()
		return ErrClosed
	case q.running:
		q.mu.Unlock()
		return ErrRunning
	}
	q.running = true
	// Added under mu so that a concurrent Close, which sets closed under mu
	// before it waits, never races wg.Add with wg.Wait.
	q.wg.Add(q.workers)
	q.mu.Unlock()

	for i := 0; i < q.workers; i++ {
		go q.work()
	}
	select {
	case <-ctx.Done():
		// No worker will take another message, so refuse new ones.
		q.mu.Lock()
		q.closed = true
		q.mu.Unlock()
		q.stop()
	case <-q.quit:
	}
	q.wg.Wait()
	return nil
}

func (q *inProcess) Close(ctx context.Context) error {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.stop()

	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		q.abort()
		return nil
	case <-ctx.Done():
		q.abort()
		<-done
		return ctx.Err()
	}
}

func (q *inProcess) isClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

func (q *inProcess) stop() { q.quitOnce.Do(func() { close(q.quit) }) }

// work takes messages until quit is closed. A message is finished before quit
// is checked again; messages still in the channel are dropped (AD-14). A
// message taken in the instant quit closes is still finished, and Close waits
// for it like any other in-flight send.
func (q *inProcess) work() {
	defer q.wg.Done()
	for {
		select {
		case <-q.quit:
			return
		default:
		}
		select {
		case <-q.quit:
			return
		case id := <-q.ch:
			q.release()
			q.deliver(id)
		}
	}
}

// deliver moves one message queued → sending → sent / failed, retrying
// transient failures with backoff in between (AD-2, AD-5, AD-6, AD-13). When
// Close aborts it mid-way the message is left in sending.
func (q *inProcess) deliver(id string) {
	m, err := q.store.Transition(id, store.StatusQueued, store.StatusSending, store.Patch{})
	if err != nil {
		slog.Warn("queue: claim failed", "message_id", id, "error_class", "transition")
		return
	}
	out := provider.OutboundMessage{ID: m.ID, TenantID: m.TenantID, To: m.To, Body: m.Body}
	for attempt := 1; ; attempt++ {
		receipt, err := q.prov.Send(q.sendCtx, out)
		switch {
		case err == nil:
			q.finish(m, store.StatusSent, store.Patch{Attempts: attempt, ProviderRef: receipt.ProviderRef})
			return
		case q.sendCtx.Err() != nil:
			return
		case !errors.Is(err, provider.ErrTransient):
			// Unclassified errors count as permanent (AD-1).
			q.finish(m, store.StatusFailed, store.Patch{Attempts: attempt, FailureReason: ReasonPermanent})
			return
		case attempt == MaxAttempts:
			q.finish(m, store.StatusFailed, store.Patch{Attempts: attempt, FailureReason: ReasonRetriesExhausted})
			return
		}
		if err := q.wait(q.sendCtx, backoff[attempt-1]); err != nil {
			return
		}
	}
}

// finish writes the terminal outcome in one compare-and-set from sending.
func (q *inProcess) finish(m store.Message, to store.Status, p store.Patch) {
	if _, err := q.store.Transition(m.ID, store.StatusSending, to, p); err != nil {
		slog.Warn("queue: transition failed", "message_id", m.ID, "tenant_id", m.TenantID,
			"status", string(to), "attempts", p.Attempts, "provider", q.prov.Name(), "error_class", "transition")
	}
}
