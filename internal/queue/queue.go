package queue

import (
	"context"
	"errors"
	"sync"

	"example.com/sms-platform/internal/store"
)

// Capacity is the bounded queue size (AD-14).
const Capacity = 1000

var (
	// ErrQueueFull is returned by Submit when no slot is free; no message is kept.
	ErrQueueFull = errors.New("queue: full")
	// ErrClosed is returned by Submit after Close.
	ErrClosed = errors.New("queue: closed")
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
	// Run blocks until ctx ends. Sending workers arrive in Story 1.6.
	Run(ctx context.Context) error
	// Close stops accepting messages; unsent ones are dropped (AD-14).
	Close(ctx context.Context) error
}

type inProcess struct {
	store store.System
	cap   int

	mu       sync.Mutex
	reserved int
	closed   bool
	ch       chan string
}

// NewInProcess returns the in-process queue with Capacity slots.
func NewInProcess(st store.System) Queue { return newInProcess(st, Capacity) }

func newInProcess(st store.System, capacity int) *inProcess {
	return &inProcess{store: st, cap: capacity, ch: make(chan string, capacity)}
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
	<-ctx.Done()
	return nil
}

func (q *inProcess) Close(context.Context) error {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	return nil
}
