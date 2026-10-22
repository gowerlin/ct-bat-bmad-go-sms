package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/store"
)

// SignatureHeader carries the HMAC-SHA256 of the raw notification body (D013).
const SignatureHeader = "X-Signature"

// signaturePrefix precedes the lowercase hex digest in SignatureHeader.
const signaturePrefix = "sha256="

// MaxAttempts is how many times one notification is posted: the first try
// plus len(backoff) retries, the same rule as sending (AD-13).
const MaxAttempts = 4

// backoff is the wait before each retry, in order (AD-13).
var backoff = [MaxAttempts - 1]time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

// ReasonUndelivered is the failure_reason written when the supplier reports
// undelivered (AD-5 requires one with every sent → undelivered).
const ReasonUndelivered = "供應商回報未送達"

var (
	// ErrUnknownProvider is a callback path naming no registered provider.
	ErrUnknownProvider = errors.New("webhook: unknown provider")
	// ErrUnauthorized is a callback the provider's ParseCallback rejected as
	// unauthenticated (provider.ErrUnauthorized).
	ErrUnauthorized = errors.New("webhook: unauthorized callback")
	// ErrInvalidReport is a callback the provider could not decode, or one
	// reporting a status other than delivered / undelivered.
	ErrInvalidReport = errors.New("webhook: invalid delivery report")
	// ErrUnknownMessage is a report whose ProviderRef matches no message that
	// has reached sent (a callback that arrives before sent is written).
	ErrUnknownMessage = errors.New("webhook: no message for provider reference")
)

// Sign returns the SignatureHeader value for body under secret:
// "sha256=" followed by the lowercase hex HMAC-SHA256 of the raw bytes.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether header is the signature of body under secret. The
// comparison is constant-time (hmac.Equal).
func Verify(secret string, body []byte, header string) bool {
	got, ok := strings.CutPrefix(header, signaturePrefix)
	if !ok {
		return false
	}
	gotMAC, err := hex.DecodeString(got)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(gotMAC, mac.Sum(nil))
}

// Targets resolves a tenant's webhook destination and signing secret; ""
// for url means the tenant gets no webhooks. *tenant.Registry satisfies it.
type Targets interface {
	Callback(tenantID string) (url, secret string)
}

// Outcome is what a callback did to its message.
type Outcome int

const (
	// Applied means the callback moved the message to a new status (204).
	Applied Outcome = iota + 1
	// Duplicate means the message was already terminal, so nothing changed
	// and nothing was enqueued (200, AD-7).
	Duplicate
)

// Inbound applies supplier callbacks: ParseCallback → ProviderRef lookup →
// sent → delivered / undelivered, enqueueing the tenant notification in the
// same atomic store operation (AD-3, AD-5, AD-7).
type Inbound struct {
	store     store.System
	targets   Targets
	providers map[string]provider.Provider
}

// NewInbound returns an Inbound accepting callbacks for the given providers,
// keyed by Name.
func NewInbound(st store.System, targets Targets, providers ...provider.Provider) *Inbound {
	in := &Inbound{store: st, targets: targets, providers: make(map[string]provider.Provider, len(providers))}
	for _, p := range providers {
		in.providers[p.Name()] = p
	}
	return in
}

// Receive handles one callback request addressed to provider name. Errors
// are ErrUnknownProvider, ErrUnauthorized, ErrInvalidReport,
// ErrUnknownMessage, or an unexpected store error.
func (in *Inbound) Receive(name string, r *http.Request) (Outcome, error) {
	p, ok := in.providers[name]
	if !ok {
		return 0, ErrUnknownProvider
	}
	rep, err := p.ParseCallback(r)
	if errors.Is(err, provider.ErrUnauthorized) {
		return 0, ErrUnauthorized
	}
	if err != nil {
		return 0, ErrInvalidReport
	}
	out, err := in.Apply(rep)
	if err == nil && out == Applied {
		slog.Info("webhook: status updated", "provider", name, "status", string(rep.Status))
	}
	return out, err
}

// Apply moves the message reported by rep from sent to rep.Status. A message
// that is already terminal, by this or an earlier callback, is a Duplicate
// and stays as it is.
func (in *Inbound) Apply(rep provider.DeliveryReport) (Outcome, error) {
	var to store.Status
	var patch store.Patch
	switch rep.Status {
	case provider.StatusDelivered:
		to = store.StatusDelivered
	case provider.StatusUndelivered:
		to, patch.FailureReason = store.StatusUndelivered, ReasonUndelivered
	default:
		return 0, ErrInvalidReport
	}
	m, err := in.store.GetByProviderRef(rep.ProviderRef)
	if errors.Is(err, store.ErrNotFound) {
		return 0, ErrUnknownMessage
	}
	if err != nil {
		return 0, err
	}
	// A lost compare-and-set means another callback moved the message first;
	// reread once and report what it found.
	for try := 0; try < 2; try++ {
		if store.IsTerminal(m.Status) {
			return Duplicate, nil
		}
		if m.Status != store.StatusSent {
			return 0, ErrUnknownMessage
		}
		transition := in.store.Transition
		if url, _ := in.targets.Callback(m.TenantID); url != "" {
			transition = in.store.TransitionAndNotify
		}
		if _, err = transition(m.ID, store.StatusSent, to, patch); err == nil {
			return Applied, nil
		}
		if !errors.Is(err, store.ErrIllegalTransition) {
			return 0, err
		}
		if m, err = in.store.GetByID(m.ID); errors.Is(err, store.ErrNotFound) {
			return 0, ErrUnknownMessage
		} else if err != nil {
			return 0, err
		}
	}
	return 0, err
}

// Outbox is the store view the Dispatcher needs; *store.Memory satisfies it.
// It cannot change a Message, so a notification never alters its status.
type Outbox interface {
	ClaimNext(ctx context.Context) (store.Notification, error)
	FinishNotification(seq int64, state store.NotificationState) error
}

// Doer sends one HTTP request. cmd/smsd injects its allow-listed
// *http.Client (AD-9); this package never builds one.
type Doer interface {
	Do(r *http.Request) (*http.Response, error)
}

// Dispatcher posts outbox entries to tenants, one at a time in enqueue order,
// so a message's notifications go out in the order they were produced (AD-3).
type Dispatcher struct {
	outbox  Outbox
	targets Targets
	client  Doer
	wait    func(ctx context.Context, d time.Duration) error
}

// NewDispatcher returns a Dispatcher. wait must block for d or until ctx
// ends; nil uses a real timer.
func NewDispatcher(o Outbox, targets Targets, client Doer, wait func(context.Context, time.Duration) error) *Dispatcher {
	if wait == nil {
		wait = timerWait
	}
	return &Dispatcher{outbox: o, targets: targets, client: client, wait: wait}
}

// timerWait is the production wait: a real timer that honours ctx.
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

// Run delivers notifications until ctx ends. It is the single Dispatcher
// goroutine; an entry being delivered when ctx ends is left delivering.
func (d *Dispatcher) Run(ctx context.Context) error {
	for {
		n, err := d.outbox.ClaimNext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		d.deliver(ctx, n)
	}
}

// payload is the fixed notification body (AD-3).
type payload struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// deliver posts n, retrying failures with backoff in between, and records
// done or abandoned. Abandoning only logs: Message.status is not touched.
func (d *Dispatcher) deliver(ctx context.Context, n store.Notification) {
	log := slog.With("message_id", n.MessageID, "tenant_id", n.TenantID, "status", string(n.Status))
	url, secret := d.targets.Callback(n.TenantID)
	if url == "" {
		log.Warn("webhook: notification abandoned", "attempts", 0, "error_class", "no_callback_url")
		d.finish(log, n, store.NotificationAbandoned)
		return
	}
	body, err := json.Marshal(payload{ID: n.MessageID, Status: string(n.Status)})
	if err != nil {
		log.Error("webhook: notification abandoned", "attempts", 0, "error_class", "encode")
		d.finish(log, n, store.NotificationAbandoned)
		return
	}
	sig := Sign(secret, body)
	for attempt := 1; ; attempt++ {
		class := d.post(ctx, url, body, sig)
		switch {
		case class == "":
			d.finish(log, n, store.NotificationDone)
			return
		case ctx.Err() != nil:
			return
		case attempt == MaxAttempts:
			log.Warn("webhook: notification abandoned", "attempts", attempt, "error_class", class)
			d.finish(log, n, store.NotificationAbandoned)
			return
		}
		if err := d.wait(ctx, backoff[attempt-1]); err != nil {
			return
		}
	}
}

// post sends one attempt and returns "" on a 2xx, otherwise an error class
// safe to log (no URL, body or secret).
func (d *Dispatcher) post(ctx context.Context, url string, body []byte, sig string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "request"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, sig)
	resp, err := d.client.Do(req)
	if err != nil {
		return "transport"
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Sprintf("http_%d", resp.StatusCode)
	}
	return ""
}

func (d *Dispatcher) finish(log *slog.Logger, n store.Notification, state store.NotificationState) {
	if err := d.outbox.FinishNotification(n.Seq, state); err != nil {
		log.Warn("webhook: outbox finish failed", "error_class", "outbox")
	}
}
