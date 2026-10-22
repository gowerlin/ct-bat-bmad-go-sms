// Package providertest is the contract every provider.Provider adapter must
// pass (AD-1). Each adapter calls Run from its own _test.go with a Harness
// that knows how to make that adapter succeed, fail and receive callbacks.
//
// It imports only the provider package, never an adapter, so the same checks
// apply to mock and mitake alike.
package providertest

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"example.com/sms-platform/internal/provider"
)

// Outcome is how every Send of a provider built by Harness.New must end.
type Outcome int

const (
	Accept        Outcome = iota // Send succeeds
	FailTransient                // Send fails with provider.ErrTransient
	FailPermanent                // Send fails with provider.ErrPermanent
)

// Harness adapts one provider implementation to the contract.
type Harness struct {
	// New returns a fresh provider whose Sends all end in o.
	New func(t *testing.T, o Outcome) provider.Provider

	// Callback returns a well-formed, authenticated callback reporting ref,
	// and the status it encodes.
	Callback func(t *testing.T, ref string) (*http.Request, provider.Status)

	// MalformedCallback returns an authenticated callback that ParseCallback
	// must reject (for example, one without a provider reference).
	MalformedCallback func(t *testing.T) *http.Request

	// UnauthorizedCallback returns an otherwise well-formed callback with
	// missing or wrong credentials; ParseCallback must reject it with
	// provider.ErrUnauthorized. Optional (D018): nil declares that the adapter
	// has no callback authentication, which only mock may do (a known gap
	// against AD-7). Every adapter that talks to a real supplier must set it.
	UnauthorizedCallback func(t *testing.T, ref string) *http.Request
}

// Message is what every contract case sends. Error text is checked for its
// recipient and body, which must never leak.
var Message = provider.OutboundMessage{
	ID:       "msg_contract",
	TenantID: "tenant_contract",
	To:       "+886900000099",
	Body:     "contract body 契約內容",
}

// callbackRef is the provider reference every callback case reports.
const callbackRef = "contract-ref-1"

// Run checks h's provider against the shared contract.
func Run(t *testing.T, h Harness) {
	t.Helper()
	if h.New == nil || h.Callback == nil || h.MalformedCallback == nil {
		t.Fatal("providertest: Harness.New, Callback and MalformedCallback are required")
	}

	t.Run("NameNonEmptyAndStable", func(t *testing.T) {
		a, b := h.New(t, Accept), h.New(t, Accept)
		if a.Name() == "" {
			t.Fatal("Name() is empty")
		}
		if a.Name() != a.Name() || a.Name() != b.Name() {
			t.Fatalf("Name() not stable: %q, %q, %q", a.Name(), a.Name(), b.Name())
		}
	})

	t.Run("SendAcceptedReturnsRef", func(t *testing.T) {
		r, err := h.New(t, Accept).Send(context.Background(), Message)
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if r.ProviderRef == "" {
			t.Fatal("ProviderRef is empty")
		}
		if r.AcceptedAt.IsZero() {
			t.Fatal("AcceptedAt is zero")
		}
	})

	t.Run("SendTransientWrapsErrTransient", func(t *testing.T) {
		_, err := h.New(t, FailTransient).Send(context.Background(), Message)
		checkSendError(t, err, provider.ErrTransient, provider.ErrPermanent)
	})

	t.Run("SendPermanentWrapsErrPermanent", func(t *testing.T) {
		_, err := h.New(t, FailPermanent).Send(context.Background(), Message)
		checkSendError(t, err, provider.ErrPermanent, provider.ErrTransient)
	})

	t.Run("SendCancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := h.New(t, Accept).Send(ctx, Message)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled in the chain", err)
		}
		if errors.Is(err, provider.ErrPermanent) {
			t.Fatalf("err = %v: a cancelled send must not be permanent", err)
		}
	})

	t.Run("ParseCallbackDecodesReport", func(t *testing.T) {
		req, want := h.Callback(t, callbackRef)
		rep, err := h.New(t, Accept).ParseCallback(req)
		if err != nil {
			t.Fatalf("ParseCallback: %v", err)
		}
		if rep.ProviderRef != callbackRef {
			t.Fatalf("ProviderRef = %q, want %q", rep.ProviderRef, callbackRef)
		}
		// A callback only ever reports delivered or undelivered (AD-7).
		if want != provider.StatusDelivered && want != provider.StatusUndelivered {
			t.Fatalf("harness callback status %q is not delivered/undelivered", want)
		}
		if rep.Status != want {
			t.Fatalf("Status = %q, want %q", rep.Status, want)
		}
		if rep.ReceivedAt.IsZero() {
			t.Fatal("ReceivedAt is zero")
		}
	})

	t.Run("ParseCallbackRejectsMalformed", func(t *testing.T) {
		_, err := h.New(t, Accept).ParseCallback(h.MalformedCallback(t))
		if err == nil {
			t.Fatal("malformed callback accepted")
		}
		if errors.Is(err, provider.ErrUnauthorized) {
			t.Fatalf("err = %v: an authenticated malformed callback is not unauthorized", err)
		}
	})

	if h.UnauthorizedCallback != nil {
		t.Run("ParseCallbackRejectsUnauthorized", func(t *testing.T) {
			_, err := h.New(t, Accept).ParseCallback(h.UnauthorizedCallback(t, callbackRef))
			if !errors.Is(err, provider.ErrUnauthorized) {
				t.Fatalf("err = %v, want ErrUnauthorized", err)
			}
		})
	}
}

// checkSendError asserts err wraps want, not other, and leaks neither the
// recipient nor the body.
func checkSendError(t *testing.T, err, want, other error) {
	t.Helper()
	if !errors.Is(err, want) || errors.Is(err, other) {
		t.Fatalf("err = %v, want %v only", err, want)
	}
	if msg := err.Error(); strings.Contains(msg, Message.To) || strings.Contains(msg, Message.Body) {
		t.Fatalf("error leaks recipient or content: %v", err)
	}
}
