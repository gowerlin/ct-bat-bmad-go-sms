package testutil

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"example.com/sms-platform/internal/provider"
)

// Response is one scripted answer of FakeProvider.Send.
type Response struct {
	Receipt provider.ProviderReceipt
	Err     error
}

// FakeProvider is a scriptable provider.Provider for E2E tests: it records
// every message, answers Send from a queue of Responses (falling back to a
// fixed success once the queue is empty) and can delay each Send so that
// timeouts and cancellation are testable. T0002 extends it for fault
// injection; T0009 feeds it DeliveryReports for callback tests.
type FakeProvider struct {
	mu        sync.Mutex
	name      string
	now       func() time.Time
	delay     time.Duration
	responses []Response
	reports   []provider.DeliveryReport
	sent      []provider.OutboundMessage
	seq       int
}

var _ provider.Provider = (*FakeProvider)(nil)

// NewFakeProvider returns a fake named "fake" that stamps receipts with now.
func NewFakeProvider(now func() time.Time) *FakeProvider {
	if now == nil {
		now = time.Now
	}
	return &FakeProvider{name: "fake", now: now}
}

// Name implements provider.Provider.
func (p *FakeProvider) Name() string { return p.name }

// Enqueue appends scripted responses consumed in order by Send.
func (p *FakeProvider) Enqueue(responses ...Response) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.responses = append(p.responses, responses...)
}

// SetDelay makes every subsequent Send wait d (or until ctx is done).
func (p *FakeProvider) SetDelay(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.delay = d
}

// EnqueueReport appends a DeliveryReport returned by the next ParseCallback.
func (p *FakeProvider) EnqueueReport(reports ...provider.DeliveryReport) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reports = append(p.reports, reports...)
}

// Sent returns a copy of every message handed to Send, in order.
func (p *FakeProvider) Sent() []provider.OutboundMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]provider.OutboundMessage, len(p.sent))
	copy(out, p.sent)
	return out
}

// Send implements provider.Provider.
func (p *FakeProvider) Send(ctx context.Context, msg provider.OutboundMessage) (provider.ProviderReceipt, error) {
	p.mu.Lock()
	delay := p.delay
	p.sent = append(p.sent, msg)
	p.seq++
	seq := p.seq
	var next *Response
	if len(p.responses) > 0 {
		r := p.responses[0]
		p.responses = p.responses[1:]
		next = &r
	}
	p.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return provider.ProviderReceipt{}, ctx.Err()
		}
	}
	if next != nil {
		return next.Receipt, next.Err
	}
	return provider.ProviderReceipt{
		ProviderRef: fmt.Sprintf("fake-%d", seq),
		AcceptedAt:  p.now(),
		Raw:         `{"accepted":true}`,
	}, nil
}

// ParseCallback implements provider.Provider: it returns the next enqueued
// report, or an error when none is scripted.
func (p *FakeProvider) ParseCallback(_ *http.Request) (provider.DeliveryReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.reports) == 0 {
		return provider.DeliveryReport{}, fmt.Errorf("fake provider: no delivery report enqueued")
	}
	r := p.reports[0]
	p.reports = p.reports[1:]
	return r, nil
}
