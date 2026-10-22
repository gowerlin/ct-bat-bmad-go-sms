// Package mock is the course-mainline SMS provider: it never talks to the
// network and always accepts a message. Fault injection (configurable delay,
// failure rate, status sequences) is added in CH3 by T0002; this skeleton
// only satisfies the provider.Provider interface.
package mock

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"example.com/sms-platform/internal/provider"
)

// Name is the identifier selected with PROVIDER=mock.
const Name = "mock"

// Provider is a provider.Provider that accepts everything.
type Provider struct {
	now func() time.Time
	seq atomic.Int64
}

var _ provider.Provider = (*Provider)(nil)

// New returns a mock provider using the wall clock.
func New() *Provider {
	return NewWithClock(time.Now)
}

// NewWithClock returns a mock provider whose timestamps come from now.
func NewWithClock(now func() time.Time) *Provider {
	return &Provider{now: now}
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return Name }

// Send implements provider.Provider: fixed success, sequential refs.
func (p *Provider) Send(ctx context.Context, msg provider.OutboundMessage) (provider.ProviderReceipt, error) {
	if err := ctx.Err(); err != nil {
		return provider.ProviderReceipt{}, err
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
