package testutil

import (
	"context"
	"sync"
	"testing"
	"time"
)

// syncTimeout bounds how long a test blocks on the system under test (a
// waiter showing up, a status being written) before it fails instead of
// hanging. It is a watchdog only: nothing sleeps or polls.
const syncTimeout = 10 * time.Second

// FakeClock is a manually advanced clock for deterministic tests of retries,
// rate limits and timestamps. Inject FakeClock.Now where code takes a
// func() time.Time and FakeClock.Wait where it takes a wait function.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*clockWaiter
	changed chan struct{} // closed and replaced whenever waiters changes
}

type clockWaiter struct {
	until time.Time
	done  chan struct{}
}

// NewFakeClock starts at a fixed, timezone-free instant so that assertions do
// not depend on the machine running the tests.
func NewFakeClock() *FakeClock {
	return &FakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), changed: make(chan struct{})}
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d and releases every Wait whose time has
// come before it returns.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.releaseLocked()
}

// Set jumps the clock to t, releasing waits as Advance does.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
	c.releaseLocked()
}

// Wait is the injected replacement for sleeping: it blocks until the clock has
// been advanced by d, or returns ctx.Err() once ctx is done.
func (c *FakeClock) Wait(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if d <= 0 {
		c.mu.Unlock()
		return nil
	}
	w := &clockWaiter{until: c.now.Add(d), done: make(chan struct{})}
	c.waiters = append(c.waiters, w)
	c.notifyLocked()
	c.mu.Unlock()

	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		c.removeLocked(w)
		c.mu.Unlock()
		return ctx.Err()
	}
}

// BlockUntilWaiters blocks until at least n Wait calls are blocked on the
// clock, so that a following Advance is sure to reach them. Advance removes
// the waits it releases, so after an Advance this waits for the next ones.
func (c *FakeClock) BlockUntilWaiters(t testing.TB, n int) {
	t.Helper()
	timeout := time.NewTimer(syncTimeout)
	defer timeout.Stop()
	for {
		c.mu.Lock()
		got, changed := len(c.waiters), c.changed
		c.mu.Unlock()
		if got >= n {
			return
		}
		select {
		case <-changed:
		case <-timeout.C:
			t.Fatalf("FakeClock: %d waiters after %v, want %d", got, syncTimeout, n)
		}
	}
}

func (c *FakeClock) releaseLocked() {
	kept := c.waiters[:0]
	for _, w := range c.waiters {
		if c.now.Before(w.until) {
			kept = append(kept, w)
		} else {
			close(w.done)
		}
	}
	if len(kept) != len(c.waiters) {
		clear(c.waiters[len(kept):])
		c.waiters = kept
		c.notifyLocked()
	}
}

func (c *FakeClock) removeLocked(w *clockWaiter) {
	for i, x := range c.waiters {
		if x == w {
			c.waiters = append(c.waiters[:i], c.waiters[i+1:]...)
			c.notifyLocked()
			return
		}
	}
}

func (c *FakeClock) notifyLocked() {
	close(c.changed)
	c.changed = make(chan struct{})
}
