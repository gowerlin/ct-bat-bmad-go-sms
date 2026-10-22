package testutil

import (
	"context"
	"sync"
	"time"
)

// FakeClock is a manually advanced clock for deterministic tests of retries,
// rate limits and timestamps. Inject FakeClock.Now where code takes a
// func() time.Time.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock starts at a fixed, timezone-free instant so that assertions do
// not depend on the machine running the tests.
func NewFakeClock() *FakeClock {
	return &FakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward by d.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// Wait is the injected replacement for sleeping: it advances the clock by d
// without blocking, or returns ctx.Err() if ctx is already done.
func (c *FakeClock) Wait(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.Advance(d)
	return nil
}

// Set jumps the clock to t.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}
