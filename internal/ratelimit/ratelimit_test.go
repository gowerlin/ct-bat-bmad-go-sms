package ratelimit

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newLimiter(limits map[string]int) (*Limiter, *clock) {
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	return New(c.now, func(id string) int { return limits[id] }), c
}

func retryAfter(t *testing.T, err error) int {
	t.Helper()
	var le *LimitedError
	if !errors.As(err, &le) {
		t.Fatalf("err = %v, want *LimitedError", err)
	}
	if !errors.Is(err, ErrLimited) {
		t.Fatal("errors.Is(err, ErrLimited) = false")
	}
	return le.RetryAfter
}

func TestBucketStartsFullThenRejects(t *testing.T) {
	l, _ := newLimiter(map[string]int{"t": 3})
	for i := 0; i < 3; i++ {
		if err := l.Allow("t", "k"); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	// empty bucket, refill 3/60 per second: next token in 20s
	if got := retryAfter(t, l.Allow("t", "k")); got != 20 {
		t.Fatalf("RetryAfter = %d, want 20", got)
	}
}

func TestRefillsContinuously(t *testing.T) {
	l, c := newLimiter(map[string]int{"t": 3})
	for i := 0; i < 3; i++ {
		_ = l.Allow("t", "k")
	}
	c.advance(5 * time.Second) // 0.25 token: 15s to go
	if got := retryAfter(t, l.Allow("t", "k")); got != 15 {
		t.Fatalf("RetryAfter after 5s = %d, want 15", got)
	}
	c.advance(15 * time.Second)
	if err := l.Allow("t", "k"); err != nil {
		t.Fatalf("after 20s: %v", err)
	}
	if err := l.Allow("t", "k"); err == nil {
		t.Fatal("second request after one refilled token was allowed")
	}
}

func TestRefillCapsAtCapacity(t *testing.T) {
	l, c := newLimiter(map[string]int{"t": 3})
	_ = l.Allow("t", "k")
	c.advance(time.Hour)
	for i := 0; i < 3; i++ {
		if err := l.Allow("t", "k"); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	if err := l.Allow("t", "k"); err == nil {
		t.Fatal("bucket held more than its capacity")
	}
}

func TestUnsetLimitUsesDefault(t *testing.T) {
	for _, limit := range []int{0, -5} {
		l, _ := newLimiter(map[string]int{"t": limit})
		for i := 0; i < DefaultPerMinute; i++ {
			if err := l.Allow("t", "k"); err != nil {
				t.Fatalf("limit %d, request %d: %v", limit, i+1, err)
			}
		}
		// default 60/min refills one token per second
		if got := retryAfter(t, l.Allow("t", "k")); got != 1 {
			t.Fatalf("limit %d: RetryAfter = %d, want 1", limit, got)
		}
	}
}

func TestBucketsArePerKey(t *testing.T) {
	l, _ := newLimiter(map[string]int{"a": 1, "b": 1})
	if err := l.Allow("a", "k1"); err != nil {
		t.Fatal(err)
	}
	if err := l.Allow("a", "k1"); err == nil {
		t.Fatal("k1 not limited")
	}
	// same tenant, another key: own bucket with the tenant's limit
	if err := l.Allow("a", "k2"); err != nil {
		t.Fatalf("k2: %v", err)
	}
	if err := l.Allow("b", "k3"); err != nil {
		t.Fatalf("k3: %v", err)
	}
}

func TestLoweredLimitShrinksBucket(t *testing.T) {
	limits := map[string]int{"t": 10}
	l, _ := newLimiter(limits)
	_ = l.Allow("t", "k") // 9 left
	limits["t"] = 2
	for i := 0; i < 2; i++ {
		if err := l.Allow("t", "k"); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	if err := l.Allow("t", "k"); err == nil {
		t.Fatal("bucket kept tokens above the lowered limit")
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	cases := []struct {
		wait time.Duration
		want int
	}{
		{0, 1},
		{time.Millisecond, 1},
		{time.Second, 1},
		{time.Second + time.Nanosecond, 2},
		{1500 * time.Millisecond, 2},
		{20 * time.Second, 20},
		{59*time.Second + 999*time.Millisecond, 60},
	}
	for _, c := range cases {
		if got := RetryAfterSeconds(c.wait); got != c.want {
			t.Errorf("RetryAfterSeconds(%v) = %d, want %d", c.wait, got, c.want)
		}
	}
}

func TestConcurrentAllowNeverOverspends(t *testing.T) {
	l, _ := newLimiter(map[string]int{"t": 50})
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
	)
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("t", "k") == nil {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 50 {
		t.Fatalf("allowed %d of 200, want 50", allowed)
	}
}
