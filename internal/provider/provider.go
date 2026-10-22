package provider

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Provider is the SMS supplier adapter (COURSE-PLAN § 6.2). The interface is
// finalised in CH5; this is the draft every adapter implements.
//
// Implementations: internal/provider/mock (course mainline) and, from CH5,
// internal/provider/mitake (Mitake-style HTTP adapter against a fake server).
type Provider interface {
	// Send hands one message to the supplier and returns its receipt.
	// It must honour ctx cancellation and must not retry: retries belong to
	// the queue.
	Send(ctx context.Context, msg OutboundMessage) (ProviderReceipt, error)

	// ParseCallback decodes a supplier status callback
	// (POST /v1/providers/{name}/callbacks) into a DeliveryReport.
	ParseCallback(r *http.Request) (DeliveryReport, error)

	// Name is the identifier used in PROVIDER=… and in the callback path.
	Name() string
}

// Sentinel errors an adapter wraps (AD-1). Unclassified errors count as
// ErrPermanent. Wrapped reason codes must not contain numbers or content.
var (
	ErrTransient    = errors.New("provider: transient failure")
	ErrPermanent    = errors.New("provider: permanent failure")
	ErrUnauthorized = errors.New("provider: unauthorized callback")
)

// OutboundMessage is what the platform asks a provider to send. The body is
// already rendered (template variables substituted) by the time it gets here.
type OutboundMessage struct {
	ID       string // platform message id
	TenantID string
	To       string // E.164; demos use the reserved +8869000000xx range
	Body     string
}

// ProviderReceipt is the supplier's acknowledgement of Send.
type ProviderReceipt struct {
	ProviderRef string    // supplier-side id, stored as Message.provider_ref
	AcceptedAt  time.Time // when the supplier accepted it
	Raw         string    // raw supplier response, for debugging only
}

// Status is the delivery state a supplier can report. The platform's own
// Message.status (queued / sending / …) is a superset owned by the store.
type Status string

const (
	StatusSent        Status = "sent"
	StatusDelivered   Status = "delivered"
	StatusUndelivered Status = "undelivered"
	StatusFailed      Status = "failed"
)

// DeliveryReport is a decoded supplier callback (COURSE-PLAN § 5.3
// DeliveryReport). The mapping ProviderRef → message_id is done by the
// webhook package, not by the provider.
type DeliveryReport struct {
	ProviderRef string
	Status      Status
	Raw         string
	ReceivedAt  time.Time
}
