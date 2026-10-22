// Package mitake is a Mitake-STYLE SMS adapter (CH5, D014, D016, D017).
//
// SIMULATION ONLY: it imitates the shape publicly described for Mitake's HTTP
// API (form POST, plain-text reply) but is not the real specification and
// claims no field-by-field fidelity with the real service. It only ever talks
// to a fake endpoint (httptest.Server in tests); usernames, passwords and
// secrets are fake and no real endpoint is ever called.
//
// The *http.Client is injected by the assembly root (AD-9) and the clock by
// the caller (AD-10). Send never retries (AD-1): it classifies the outcome as
// provider.ErrTransient or provider.ErrPermanent and the queue decides.
package mitake

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"example.com/sms-platform/internal/provider"
)

// Name is the identifier selected with PROVIDER=mitake.
const Name = "mitake"

const (
	// SendPath is the simulated send endpoint, relative to Config.BaseURL (D014).
	SendPath = "/SmSendPost.asp"
	// SecretHeader carries the shared callback secret (D017).
	SecretHeader = "X-Mitake-Secret"

	// DoneTimeLayout is the layout of the callback field donetime (D017).
	DoneTimeLayout = "20060102150405"

	maxResponseBytes = 64 << 10
	maxCallbackBytes = 64 << 10
)

// doneTimeZone is the fixed zone of donetime. A fixed offset, not
// time.LoadLocation, so hosts without tzdata (Windows) work (D017).
var doneTimeZone = time.FixedZone("UTC+8", 8*60*60)

// transientStatusCodes are the simulated send statuscodes that mean "try
// again later" (D016): t = supplier busy, u = supplier paused. Every other
// statuscode except 1 is a permanent failure.
var transientStatusCodes = map[string]bool{"t": true, "u": true}

// callbackStatuses maps a callback statuscode to a delivery status (D017).
// Per AD-7 a callback only reports delivered or undelivered; any other value,
// including 1 (sent), is rejected rather than guessed.
var callbackStatuses = map[string]provider.Status{
	"4": provider.StatusDelivered,
	"5": provider.StatusUndelivered,
	"6": provider.StatusUndelivered,
	"8": provider.StatusUndelivered,
}

// Config is the adapter configuration. All fields are required. The values
// are fake in every test and example; none of them may be logged.
type Config struct {
	BaseURL        string // fake endpoint, e.g. an httptest.Server URL
	Username       string
	Password       string
	CallbackSecret string // compared with the SecretHeader of callbacks
}

// Provider is the Mitake-style provider.Provider.
type Provider struct {
	cfg     Config
	sendURL string
	client  *http.Client
	now     func() time.Time
}

var _ provider.Provider = (*Provider)(nil)

// New validates cfg and returns the adapter. client is the restricted client
// built by the assembly root (AD-9); now is the injected clock (AD-10).
// Errors name the missing setting, never its value.
func New(cfg Config, client *http.Client, now func() time.Time) (*Provider, error) {
	switch {
	case cfg.BaseURL == "":
		return nil, errors.New("mitake: base URL is required")
	case cfg.Username == "":
		return nil, errors.New("mitake: username is required")
	case cfg.Password == "":
		return nil, errors.New("mitake: password is required")
	case cfg.CallbackSecret == "":
		return nil, errors.New("mitake: callback secret is required")
	case client == nil:
		return nil, errors.New("mitake: http client is required")
	case now == nil:
		return nil, errors.New("mitake: clock is required")
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("mitake: base URL must be an absolute http(s) URL")
	}
	return &Provider{
		cfg:     cfg,
		sendURL: strings.TrimSuffix(cfg.BaseURL, "/") + SendPath,
		client:  client,
		now:     now,
	}, nil
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return Name }

// Send implements provider.Provider: one form POST to {BaseURL}/SmSendPost.asp
// (D014), classified per D016. Errors carry only a reason (HTTP status or
// statuscode), never the password, recipient or content.
func (p *Provider) Send(ctx context.Context, msg provider.OutboundMessage) (provider.ProviderReceipt, error) {
	if err := ctx.Err(); err != nil {
		return provider.ProviderReceipt{}, contextError(err)
	}
	form := url.Values{
		"username":   {p.cfg.Username},
		"password":   {p.cfg.Password},
		"dstaddr":    {msg.To},
		"smbody":     {msg.Body},
		"CharsetURL": {"UTF-8"},
		"clientid":   {msg.ID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.sendURL, strings.NewReader(form.Encode()))
	if err != nil {
		return provider.ProviderReceipt{}, fmt.Errorf("mitake: build request: %w", provider.ErrPermanent)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return provider.ProviderReceipt{}, contextError(ctxErr)
		}
		// The *url.Error is not wrapped: only the class crosses the boundary.
		return provider.ProviderReceipt{}, fmt.Errorf("mitake: network error: %w", provider.ErrTransient)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return provider.ProviderReceipt{}, contextError(ctxErr)
		}
		return provider.ProviderReceipt{}, fmt.Errorf("mitake: read response: %w", provider.ErrTransient)
	}

	switch code := resp.StatusCode; {
	case code >= 500 || code == http.StatusTooManyRequests:
		return provider.ProviderReceipt{}, fmt.Errorf("mitake: http %d: %w", code, provider.ErrTransient)
	case code < 200 || code > 299:
		// Other 4xx, and anything else unclassified (AD-1), is permanent.
		return provider.ProviderReceipt{}, fmt.Errorf("mitake: http %d: %w", code, provider.ErrPermanent)
	}

	reply, ok := parseSendReply(string(raw))
	if !ok {
		// The supplier may have accepted the message: retrying risks a
		// duplicate SMS, so an unparsable 2xx is permanent (D016).
		return provider.ProviderReceipt{}, fmt.Errorf("mitake: unparsable response: %w", provider.ErrPermanent)
	}
	switch {
	case reply.statusCode == "1":
		return provider.ProviderReceipt{
			ProviderRef: reply.msgID,
			AcceptedAt:  p.now(),
			Raw:         string(raw),
		}, nil
	case transientStatusCodes[reply.statusCode]:
		return provider.ProviderReceipt{}, fmt.Errorf("mitake: statuscode %s: %w", reply.statusCode, provider.ErrTransient)
	default:
		return provider.ProviderReceipt{}, fmt.Errorf("mitake: statuscode %s: %w", reply.statusCode, provider.ErrPermanent)
	}
}

// contextError keeps the context error in the chain and also marks it
// transient, so the queue sees a classified error (D016).
func contextError(err error) error {
	return fmt.Errorf("mitake: %w: %w", provider.ErrTransient, err)
}

type sendReply struct {
	msgID      string
	statusCode string
}

// parseSendReply reads the plain-text reply: a first line "[1]", then
// key=value lines. statuscode is always required; msgid only on success.
// The statuscode is accepted only if it is a short alphanumeric token, so
// error text never echoes arbitrary supplier content.
func parseSendReply(raw string) (sendReply, bool) {
	var lines []string
	for _, l := range strings.Split(raw, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 || lines[0] != "[1]" {
		return sendReply{}, false
	}
	var r sendReply
	for _, l := range lines[1:] {
		k, v, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		switch k {
		case "msgid":
			r.msgID = v
		case "statuscode":
			r.statusCode = v
		}
	}
	if !isToken(r.statusCode) || (r.statusCode == "1" && r.msgID == "") {
		return sendReply{}, false
	}
	return r, true
}

func isToken(s string) bool {
	if s == "" || len(s) > 4 {
		return false
	}
	for _, c := range s {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z') {
			return false
		}
	}
	return true
}

// ParseCallback implements provider.Provider (D017): a form POST with msgid,
// statuscode and optional donetime, authenticated by SecretHeader. The secret
// is checked first, in constant time, so an unauthenticated caller learns
// nothing about the payload rules.
func (p *Provider) ParseCallback(r *http.Request) (provider.DeliveryReport, error) {
	got := r.Header.Get(SecretHeader)
	if subtle.ConstantTimeCompare([]byte(got), []byte(p.cfg.CallbackSecret)) != 1 {
		return provider.DeliveryReport{}, fmt.Errorf("mitake: callback secret missing or wrong: %w", provider.ErrUnauthorized)
	}
	if r.Method != http.MethodPost {
		return provider.DeliveryReport{}, fmt.Errorf("mitake: callback method: %w", provider.ErrPermanent)
	}
	if ct, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";"); strings.TrimSpace(ct) != "application/x-www-form-urlencoded" {
		return provider.DeliveryReport{}, fmt.Errorf("mitake: callback content type: %w", provider.ErrPermanent)
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCallbackBytes+1))
	if err != nil || len(body) > maxCallbackBytes {
		return provider.DeliveryReport{}, fmt.Errorf("mitake: callback body: %w", provider.ErrPermanent)
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return provider.DeliveryReport{}, fmt.Errorf("mitake: callback form: %w", provider.ErrPermanent)
	}
	ref := form.Get("msgid")
	if ref == "" {
		return provider.DeliveryReport{}, fmt.Errorf("mitake: callback missing msgid: %w", provider.ErrPermanent)
	}
	status, ok := callbackStatuses[form.Get("statuscode")]
	if !ok {
		return provider.DeliveryReport{}, fmt.Errorf("mitake: callback statuscode not accepted: %w", provider.ErrPermanent)
	}
	received, err := time.ParseInLocation(DoneTimeLayout, form.Get("donetime"), doneTimeZone)
	if err != nil {
		received = p.now()
	}
	return provider.DeliveryReport{
		ProviderRef: ref,
		Status:      status,
		Raw:         string(body),
		ReceivedAt:  received,
	}, nil
}
