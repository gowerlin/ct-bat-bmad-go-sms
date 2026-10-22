package ratelimit

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// DefaultPerMinute is the limit used when a tenant has none set (<= 0)
// (AD-12; T0005 Q-1).
const DefaultPerMinute = 60

// ErrLimited is the sentinel for a rejected request; the concrete error is a
// *LimitedError carrying Retry-After.
var ErrLimited = errors.New("ratelimit: limited")

// LimitedError reports a rejected request and how long the caller should wait.
type LimitedError struct {
	// RetryAfter is whole seconds until the next token, at least 1.
	RetryAfter int
}

func (e *LimitedError) Error() string {
	return fmt.Sprintf("ratelimit: limited, retry after %ds", e.RetryAfter)
}

// Is makes errors.Is(err, ErrLimited) true for a *LimitedError.
func (e *LimitedError) Is(target error) bool { return target == ErrLimited }

// RetryAfterSeconds rounds the wait until the next token to the Retry-After
// value: max(1, ceil(seconds)) (AD-12; T0005 Q-2). Rounding up means a client
// that waits that long finds a token.
func RetryAfterSeconds(wait time.Duration) int {
	return max(1, int(math.Ceil(wait.Seconds())))
}

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter is a token bucket per API key (AD-12). A bucket's capacity is its
// tenant's per-minute limit and it refills continuously at limit/60 tokens per
// second; a new bucket starts full.
type Limiter struct {
	now      func() time.Time
	limitFor func(tenantID string) int

	mu      sync.Mutex
	buckets map[string]*bucket
}

// New returns a Limiter reading time from now (AD-10) and each tenant's
// per-minute limit from limitFor; a limit <= 0 means DefaultPerMinute. Where
// the limit is stored is not this package's concern (tenant owns it).
func New(now func() time.Time, limitFor func(tenantID string) int) *Limiter {
	return &Limiter{now: now, limitFor: limitFor, buckets: map[string]*bucket{}}
}

// Allow takes one token from key's bucket, sized by tenantID's limit. key
// identifies the API key (callers pass a hash, never the plaintext key). It
// returns nil, or a *LimitedError when the bucket is empty.
func (l *Limiter) Allow(tenantID, key string) error {
	perMin := l.limitFor(tenantID)
	if perMin <= 0 {
		perMin = DefaultPerMinute
	}
	capacity := float64(perMin)
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: capacity, last: now}
		l.buckets[key] = b
	}
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens += elapsed.Seconds() * capacity / 60
		b.last = now
	}
	// A lowered limit shrinks the bucket right away.
	b.tokens = min(b.tokens, capacity)

	if b.tokens >= 1 {
		b.tokens--
		return nil
	}
	wait := time.Duration((1 - b.tokens) * 60 / capacity * float64(time.Second))
	return &LimitedError{RetryAfter: RetryAfterSeconds(wait)}
}
