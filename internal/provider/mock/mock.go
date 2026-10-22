// Package mock is the course-mainline SMS provider: it never talks to the
// network and accepts every message unless a test injects a fault (transient
// or permanent failure, timeout, delay; see the Fail*/Timeout/Delay methods).
// With EmitCallbacks it also reports each accepted message delivered through
// an injected CallbackSink instead of the network.
package mock

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	callbacks *emitter // nil: Send emits no callbacks
}

var _ provider.Provider = (*Provider)(nil)

// CallbackPath is the platform route the mock's callbacks are addressed to.
const CallbackPath = "/v1/providers/" + Name + "/callbacks"

// CallbackSink receives the callbacks the mock emits (AD-6): a form POST to
// CallbackPath that ParseCallback accepts. cmd/smsd hands each one to the
// platform handler, so a mock callback takes the same route and ParseCallback
// as a supplier's HTTP callback.
type CallbackSink func(ctx context.Context, r *http.Request)

type emitter struct {
	ctx   context.Context
	sink  CallbackSink
	wait  func(context.Context, time.Duration) error
	delay time.Duration
}

// EmitCallbacks makes every later successful Send report delivered for its
// ref to sink, from its own goroutine once wait(ctx, delay) returns, so the
// queue has written sent before the callback arrives. wait must block for d
// or until ctx ends; nil uses a real timer. ctx bounds every pending
// callback. A nil sink turns emitting off.
func (p *Provider) EmitCallbacks(ctx context.Context, sink CallbackSink, wait func(context.Context, time.Duration) error, delay time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if sink == nil {
		p.callbacks = nil
		return
	}
	if wait == nil {
		wait = timerWait
	}
	p.callbacks = &emitter{ctx: ctx, sink: sink, wait: wait, delay: delay}
}

// timerWait is the default EmitCallbacks wait: a real timer that honours ctx.
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

// emit delivers one delivered callback for ref after the configured wait.
func (e *emitter) emit(ref string) {
	if err := e.wait(e.ctx, e.delay); err != nil {
		return
	}
	form := url.Values{"ref": {ref}, "status": {string(provider.StatusDelivered)}}
	r, err := http.NewRequestWithContext(e.ctx, http.MethodPost, CallbackPath, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	e.sink(e.ctx, r)
}

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
	ref := fmt.Sprintf("mock-%d", n)
	p.mu.Lock()
	e := p.callbacks
	p.mu.Unlock()
	if e != nil {
		go e.emit(ref)
	}
	return provider.ProviderReceipt{
		ProviderRef: ref,
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
