package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
	"example.com/sms-platform/internal/template"
	"example.com/sms-platform/internal/tenant"
	"example.com/sms-platform/internal/webhook"
)

func TestValidateMessage(t *testing.T) {
	cases := []struct {
		name, to, body string
		ok             bool
	}{
		{"ok", "+886900000001", "hi", true},
		{"70 code points", "+886900000001", strings.Repeat("字", 70), true},
		{"71 code points", "+886900000001", strings.Repeat("字", 71), false},
		{"missing to", "", "hi", false},
		{"missing body", "+886900000001", "", false},
		{"no plus", "886900000001", "hi", false},
		{"leading zero", "+0886900000001", "hi", false},
		{"too long", "+8869000000012345", "hi", false},
	}
	for _, c := range cases {
		if err := validateMessage(c.to, c.body); (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestWriteErrorUnknownIs500FixedMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, errors.New("secret internals: +886900000001"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Code != "internal_error" || got.Error.Message != internalMessage {
		t.Fatalf("envelope = %+v", got)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("leaked internals: %s", rec.Body.String())
	}
}

func TestNewHandlerRequiresDeps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing Queue/Store/Tenants")
		}
	}()
	NewHandler(Deps{})
}

func TestWriteErrorUnauthorizedIs401Envelope(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, errUnauthorized)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Code != "unauthorized" || got.Error.Message == "" {
		t.Fatalf("envelope = %+v", got)
	}
}

func TestWriteErrorFeatureMapperAppliesStatusEnvelopeAndHeader(t *testing.T) {
	orig := featureErrorMappers
	t.Cleanup(func() { featureErrorMappers = orig })
	sentinel := errors.New("feature sentinel")
	featureErrorMappers = []func(error) (*apiError, bool){
		func(err error) (*apiError, bool) {
			if !errors.Is(err, sentinel) {
				return nil, false
			}
			return &apiError{
				Status: http.StatusTooManyRequests, Code: "x", Message: "slow down",
				Header: http.Header{"Retry-After": {"7"}},
			}, true
		},
	}
	rec := httptest.NewRecorder()
	writeError(rec, sentinel)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "7" {
		t.Fatalf("Retry-After = %q", got)
	}
	var got struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Code != "x" || got.Error.Message != "slow down" {
		t.Fatalf("envelope = %+v", got)
	}
}

func TestNewHandlerRequiresTemplates(t *testing.T) {
	st := store.New(func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing Templates")
		}
	}()
	NewHandler(Deps{Queue: queue.NewInProcess(st), Store: st, Tenants: tenant.NewRegistry()})
}

func TestWriteErrorInvalidTemplateIs400InvalidTemplate(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, fmt.Errorf("create: %w", template.ErrInvalidTemplate))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Code != "invalid_template" || got.Error.Message == "" {
		t.Fatalf("envelope = %+v", got)
	}
}

// fakeCallbacks answers Receive with a fixed outcome and records the name.
type fakeCallbacks struct {
	out   webhook.Outcome
	err   error
	names []string
}

func (f *fakeCallbacks) Receive(name string, _ *http.Request) (webhook.Outcome, error) {
	f.names = append(f.names, name)
	return f.out, f.err
}

func callbackHandler(t *testing.T, cb Callbacks) http.Handler {
	t.Helper()
	st := store.New(func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	return NewHandler(Deps{
		Queue: queue.NewInProcess(st), Store: st, Tenants: tenant.NewRegistry(),
		Templates: template.NewService(st), Callbacks: cb,
	})
}

func TestProviderCallbackStatusCodes(t *testing.T) {
	cases := []struct {
		name     string
		cb       *fakeCallbacks
		status   int
		wantCode string
	}{
		{"applied", &fakeCallbacks{out: webhook.Applied}, http.StatusNoContent, ""},
		{"duplicate", &fakeCallbacks{out: webhook.Duplicate}, http.StatusOK, ""},
		{"unknown provider", &fakeCallbacks{err: webhook.ErrUnknownProvider}, http.StatusNotFound, "not_found"},
		{"unknown message", &fakeCallbacks{err: webhook.ErrUnknownMessage}, http.StatusNotFound, "not_found"},
		{"unauthorized", &fakeCallbacks{err: webhook.ErrUnauthorized}, http.StatusUnauthorized, "unauthorized"},
		{"invalid report", &fakeCallbacks{err: webhook.ErrInvalidReport}, http.StatusBadRequest, "invalid_request"},
	}
	for _, c := range cases {
		h := callbackHandler(t, c.cb)
		// No X-API-Key: the callback route is outside authenticate (AD-7).
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/providers/mock/callbacks", strings.NewReader("ref=x")))
		if rec.Code != c.status {
			t.Errorf("%s: status = %d %s, want %d", c.name, rec.Code, rec.Body, c.status)
			continue
		}
		if len(c.cb.names) != 1 || c.cb.names[0] != "mock" {
			t.Errorf("%s: Receive names = %v", c.name, c.cb.names)
		}
		if c.wantCode == "" {
			if rec.Body.Len() != 0 {
				t.Errorf("%s: body = %s", c.name, rec.Body)
			}
			continue
		}
		var got struct {
			Error struct{ Code, Message string }
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Error.Code != c.wantCode {
			t.Errorf("%s: envelope = %s (%v)", c.name, rec.Body, err)
		}
	}
}

func TestProviderCallbackRouteNeedsDeps(t *testing.T) {
	h := callbackHandler(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/providers/mock/callbacks", strings.NewReader("ref=x")))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}
