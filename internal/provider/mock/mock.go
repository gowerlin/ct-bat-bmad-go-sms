// Package mock is the course-mainline SMS provider: it never talks to the
// network and accepts every message unless a test injects a fault (transient
// or permanent failure, timeout, delay; see the Fail*/Timeout/Delay methods).
package mock

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"example.com/sms-platform/internal/provider"
)

// Name is the identifier selected with PROVIDER=mock.
const Name = "mock"

type faultMode int

const (
	faultNone faultMode = iota
	faultTransient
	faultPermanent
	faultTimeout
)

// Provider is a provider.Provider that accepts everything unless a fault is
// injected. Injection methods are safe to call concurrently with Send.
type Provider struct {
	now func() time.Time
	seq atomic.Int64

	mu        sync.Mutex
	mode      faultMode
	remaining int // sends left to fail; < 0 means until Reset
	delay     time.Duration
	after     func(time.Duration) <-chan time.Time
}

var _ provider.Provider = (*Provider)(nil)

// New returns a mock provider using the wall clock.
func New() *Provider {
	return NewWithClock(time.Now)
}

// NewWithClock returns a mock provider whose timestamps come from now.
func NewWithClock(now func() time.Time) *Provider {
	return &Provider{now: now, after: time.After}
}

// SetAfter replaces the wait used by Delay so tests need not sleep for real.
// after(d) must return a channel that fires once d has "elapsed".
func (p *Provider) SetAfter(after func(time.Duration) <-chan time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.after = after
}

// FailTransient makes the next n Sends fail with provider.ErrTransient, then
// the provider recovers. n <= 0 fails every Send until Reset.
func (p *Provider) FailTransient(n int) { p.inject(faultTransient, n) }

// FailPermanent makes every Send fail with provider.ErrPermanent until Reset.
func (p *Provider) FailPermanent() { p.inject(faultPermanent, -1) }

// Timeout makes every Send block until ctx ends and return ctx.Err(), until
// Reset.
func (p *Provider) Timeout() { p.inject(faultTimeout, -1) }

// Delay makes every Send wait d before it succeeds or fails, until Reset. The
// wait honours ctx cancellation. d <= 0 removes the delay.
func (p *Provider) Delay(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.delay = d
}

// Reset clears every injected fault and delay.
func (p *Provider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode, p.remaining, p.delay = faultNone, 0, 0
}

func (p *Provider) inject(m faultMode, n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n <= 0 {
		n = -1
	}
	p.mode, p.remaining = m, n
}

// nextFault returns the delay and fault to apply to this Send, consuming one
// count of a limited fault.
func (p *Provider) nextFault() (time.Duration, faultMode, func(time.Duration) <-chan time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	mode := p.mode
	if mode != faultNone && p.remaining > 0 {
		p.remaining--
		if p.remaining == 0 {
			p.mode = faultNone
		}
	}
	return p.delay, mode, p.after
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return Name }

// Send implements provider.Provider: sequential refs on success, or the
// injected fault. It never retries and never sleeps. Error reasons carry no
// recipient or content.
func (p *Provider) Send(ctx context.Context, msg provider.OutboundMessage) (provider.ProviderReceipt, error) {
	if err := ctx.Err(); err != nil {
		return provider.ProviderReceipt{}, err
	}
	delay, mode, after := p.nextFault()
	if delay > 0 {
		select {
		case <-after(delay):
		case <-ctx.Done():
			return provider.ProviderReceipt{}, ctx.Err()
		}
	}
	switch mode {
	case faultTransient:
		return provider.ProviderReceipt{}, fmt.Errorf("mock: injected failure: %w", provider.ErrTransient)
	case faultPermanent:
		return provider.ProviderReceipt{}, fmt.Errorf("mock: injected failure: %w", provider.ErrPermanent)
	case faultTimeout:
		<-ctx.Done()
		return provider.ProviderReceipt{}, ctx.Err()
	}
	n := p.seq.Add(1)
	return provider.ProviderReceipt{
		ProviderRef: fmt.Sprintf("mock-%d", n),
		AcceptedAt:  p.now(),
		Raw:         fmt.Sprintf(`{"id":%q,"to":%q,"accepted":true}`, msg.ID, msg.To),
	}, nil
}

// ParseCallback implements provider.Provider. The mock callback is a form or
// query with ref=<ProviderRef>&status=<Status>; status defaults to delivered.
func (p *Provider) ParseCallback(r *http.Request) (provider.DeliveryReport, error) {
	ref := r.FormValue("ref")
	if ref == "" {
		return provider.DeliveryReport{}, fmt.Errorf("mock: callback missing ref")
	}
	status := provider.Status(r.FormValue("status"))
	if status == "" {
		status = provider.StatusDelivered
	}
	return provider.DeliveryReport{
		ProviderRef: ref,
		Status:      status,
		Raw:         r.Form.Encode(),
		ReceivedAt:  p.now(),
	}, nil
}
