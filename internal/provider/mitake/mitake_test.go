package mitake

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/provider/providertest"
)

// Fake values only: no real endpoint, account or secret is used anywhere.
const (
	testUser   = "fake-user"
	testPass   = "fake-password-do-not-log"
	testSecret = "fake-callback-secret"
)

var msg = provider.OutboundMessage{ID: "msg_1", TenantID: "t1", To: "+886900000001", Body: "你好 hello"}

func fixedClock() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

// fakeServer is a Mitake-style fake endpoint that answers every request with
// status and body and records the forms it received.
type fakeServer struct {
	*httptest.Server
	mu    sync.Mutex
	forms []url.Values
}

func newFake(t *testing.T, status int, body string) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("fake: parse form: %v", err)
		}
		f.mu.Lock()
		f.forms = append(f.forms, r.PostForm)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.Close)
	return f
}

func newProvider(t *testing.T, baseURL string, client *http.Client) *Provider {
	t.Helper()
	p, err := New(Config{BaseURL: baseURL, Username: testUser, Password: testPass, CallbackSecret: testSecret}, client, fixedClock)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

const okReply = "[1]\r\nmsgid=1234567890\r\nstatuscode=1\r\nAccountPoint=98\r\n"

func callback(form url.Values, secret string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/providers/mitake/callbacks", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if secret != "" {
		r.Header.Set(SecretHeader, secret)
	}
	return r
}

func TestContract(t *testing.T) {
	providertest.Run(t, providertest.Harness{
		New: func(t *testing.T, o providertest.Outcome) provider.Provider {
			body := okReply
			switch o {
			case providertest.FailTransient:
				body = "[1]\r\nstatuscode=t\r\n"
			case providertest.FailPermanent:
				body = "[1]\r\nstatuscode=e\r\n"
			}
			f := newFake(t, http.StatusOK, body)
			return newProvider(t, f.URL, f.Client())
		},
		Callback: func(t *testing.T, ref string) (*http.Request, provider.Status) {
			return callback(url.Values{"msgid": {ref}, "statuscode": {"4"}}, testSecret), provider.StatusDelivered
		},
		MalformedCallback: func(t *testing.T) *http.Request {
			return callback(url.Values{"statuscode": {"4"}}, testSecret)
		},
		UnauthorizedCallback: func(t *testing.T, ref string) *http.Request {
			return callback(url.Values{"msgid": {ref}, "statuscode": {"4"}}, "wrong-secret")
		},
	})
}

func TestSend_PostsFormAndReturnsReceipt(t *testing.T) {
	var got *http.Request
	var form url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		_ = r.ParseForm()
		form = r.PostForm
		_, _ = w.Write([]byte(okReply))
	}))
	t.Cleanup(srv.Close)

	r, err := newProvider(t, srv.URL+"/", srv.Client()).Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Method != http.MethodPost || got.URL.Path != SendPath {
		t.Fatalf("request = %s %s", got.Method, got.URL.Path)
	}
	if ct := got.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q", ct)
	}
	want := map[string]string{
		"username": testUser, "password": testPass, "dstaddr": msg.To,
		"smbody": msg.Body, "CharsetURL": "UTF-8", "clientid": msg.ID,
	}
	for k, v := range want {
		if form.Get(k) != v {
			t.Errorf("form %s = %q, want %q", k, form.Get(k), v)
		}
	}
	if r.ProviderRef != "1234567890" || r.Raw != okReply || !r.AcceptedAt.Equal(fixedClock()) {
		t.Fatalf("receipt = %+v", r)
	}
}

func TestSend_Classification(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"statuscode t", 200, "[1]\nstatuscode=t\n", provider.ErrTransient},
		{"statuscode u", 200, "[1]\nstatuscode=u\n", provider.ErrTransient},
		{"http 500", 500, "", provider.ErrTransient},
		{"http 503", 503, okReply, provider.ErrTransient},
		{"http 429", 429, "", provider.ErrTransient},
		{"statuscode e", 200, "[1]\nstatuscode=e\n", provider.ErrPermanent},
		{"statuscode 0", 200, "[1]\nmsgid=1\nstatuscode=0\n", provider.ErrPermanent},
		{"http 400", 400, "", provider.ErrPermanent},
		{"http 401", 401, "", provider.ErrPermanent},
		{"http 404", 404, "", provider.ErrPermanent},
		{"200 empty", 200, "", provider.ErrPermanent},
		{"200 no header line", 200, "msgid=1\nstatuscode=1\n", provider.ErrPermanent},
		{"200 no statuscode", 200, "[1]\nmsgid=1\n", provider.ErrPermanent},
		{"200 success without msgid", 200, "[1]\nstatuscode=1\n", provider.ErrPermanent},
		{"200 junk statuscode", 200, "[1]\nstatuscode=<script>\n", provider.ErrPermanent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t, c.status, c.body)
			_, err := newProvider(t, f.URL, f.Client()).Send(context.Background(), msg)
			other := provider.ErrPermanent
			if c.want == provider.ErrPermanent {
				other = provider.ErrTransient
			}
			if !errors.Is(err, c.want) || errors.Is(err, other) {
				t.Fatalf("err = %v, want %v only", err, c.want)
			}
			assertNoLeak(t, err)
		})
	}
}

func TestSend_NetworkErrorIsTransient(t *testing.T) {
	f := newFake(t, 200, okReply)
	p := newProvider(t, f.URL, f.Client())
	f.Close()
	_, err := p.Send(context.Background(), msg)
	if !errors.Is(err, provider.ErrTransient) || errors.Is(err, provider.ErrPermanent) {
		t.Fatalf("err = %v, want ErrTransient", err)
	}
	assertNoLeak(t, err)
}

func TestSend_CancelledInFlightIsTransientAndKeepsCause(t *testing.T) {
	arrived, release := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		<-release // the server may not notice the client's cancel; Close needs the handler done
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) }) // runs before srv.Close
	p := newProvider(t, srv.URL, srv.Client())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := p.Send(ctx, msg); done <- err }()
	<-arrived
	cancel()
	if err := <-done; !errors.Is(err, provider.ErrTransient) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want ErrTransient and context.Canceled", err)
	}
}

func TestSend_DeadlineExceededIsTransient(t *testing.T) {
	f := newFake(t, 200, okReply)
	p := newProvider(t, f.URL, f.Client())
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	_, err := p.Send(ctx, msg)
	if !errors.Is(err, provider.ErrTransient) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want ErrTransient and DeadlineExceeded", err)
	}
	if len(f.forms) != 0 {
		t.Fatalf("fake received %d requests after the deadline", len(f.forms))
	}
}

// callbackProvider returns an adapter for ParseCallback tests; its fake
// endpoint is never called.
func callbackProvider(t *testing.T) *Provider {
	t.Helper()
	f := newFake(t, http.StatusOK, okReply)
	return newProvider(t, f.URL, f.Client())
}

func assertNoLeak(t *testing.T, err error) {
	t.Helper()
	for _, s := range []string{testPass, msg.To, msg.Body} {
		if strings.Contains(err.Error(), s) {
			t.Fatalf("error leaks %q: %v", s, err)
		}
	}
}

func TestNew_ValidatesConfigWithoutEchoingValues(t *testing.T) {
	full := Config{BaseURL: "http://127.0.0.1:1", Username: testUser, Password: testPass, CallbackSecret: testSecret}
	f := newFake(t, 200, okReply)
	cases := map[string]func(*Config){
		"no base URL":       func(c *Config) { c.BaseURL = "" },
		"no username":       func(c *Config) { c.Username = "" },
		"no password":       func(c *Config) { c.Password = "" },
		"no secret":         func(c *Config) { c.CallbackSecret = "" },
		"relative base URL": func(c *Config) { c.BaseURL = "/SmSendPost.asp" },
		"bad scheme":        func(c *Config) { c.BaseURL = "ftp://" + testPass + "@example.invalid" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := full
			mutate(&c)
			_, err := New(c, f.Client(), fixedClock)
			if err == nil {
				t.Fatal("New accepted invalid config")
			}
			if strings.Contains(err.Error(), testPass) || strings.Contains(err.Error(), testSecret) {
				t.Fatalf("error echoes a secret: %v", err)
			}
		})
	}
	if _, err := New(full, nil, fixedClock); err == nil {
		t.Fatal("New accepted nil client")
	}
	if _, err := New(full, f.Client(), nil); err == nil {
		t.Fatal("New accepted nil clock")
	}
}

func TestParseCallback_StatusMapping(t *testing.T) {
	p := callbackProvider(t)
	for code, want := range map[string]provider.Status{
		"4": provider.StatusDelivered,
		"5": provider.StatusUndelivered,
		"6": provider.StatusUndelivered,
		"8": provider.StatusUndelivered,
	} {
		rep, err := p.ParseCallback(callback(url.Values{"msgid": {"r1"}, "statuscode": {code}}, testSecret))
		if err != nil || rep.Status != want || rep.ProviderRef != "r1" {
			t.Errorf("statuscode %s: %+v, %v; want %s", code, rep, err, want)
		}
	}
	for _, code := range []string{"1", "0", "2", "9", "e", "", "delivered"} {
		_, err := p.ParseCallback(callback(url.Values{"msgid": {"r1"}, "statuscode": {code}}, testSecret))
		if !errors.Is(err, provider.ErrPermanent) || errors.Is(err, provider.ErrUnauthorized) {
			t.Errorf("statuscode %q: err = %v, want ErrPermanent", code, err)
		}
	}
}

func TestParseCallback_ReceivedAt(t *testing.T) {
	p := callbackProvider(t)
	form := url.Values{"msgid": {"r1"}, "statuscode": {"4"}, "donetime": {"20261023021500"}}
	rep, err := p.ParseCallback(callback(form, testSecret))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 22, 18, 15, 0, 0, time.UTC); !rep.ReceivedAt.Equal(want) {
		t.Fatalf("ReceivedAt = %v, want %v", rep.ReceivedAt, want)
	}
	if rep.Raw != form.Encode() {
		t.Fatalf("Raw = %q, want the form body", rep.Raw)
	}
	for _, dt := range []string{"", "2026-10-23", "20261399999999"} {
		form.Set("donetime", dt)
		rep, err := p.ParseCallback(callback(form, testSecret))
		if err != nil || !rep.ReceivedAt.Equal(fixedClock()) {
			t.Errorf("donetime %q: ReceivedAt = %v, %v; want injected Now", dt, rep.ReceivedAt, err)
		}
	}
}

func TestParseCallback_Rejects(t *testing.T) {
	p := callbackProvider(t)
	good := url.Values{"msgid": {"r1"}, "statuscode": {"4"}}

	for name, secret := range map[string]string{"missing": "", "wrong": "nope", "prefix": testSecret[:5]} {
		_, err := p.ParseCallback(callback(good, secret))
		if !errors.Is(err, provider.ErrUnauthorized) {
			t.Errorf("secret %s: err = %v, want ErrUnauthorized", name, err)
		}
		if err != nil && strings.Contains(err.Error(), testSecret) {
			t.Errorf("error echoes the secret: %v", err)
		}
	}

	get := httptest.NewRequest(http.MethodGet, "/v1/providers/mitake/callbacks?"+good.Encode(), nil)
	get.Header.Set(SecretHeader, testSecret)
	if _, err := p.ParseCallback(get); !errors.Is(err, provider.ErrPermanent) {
		t.Errorf("GET: err = %v, want ErrPermanent", err)
	}

	js := httptest.NewRequest(http.MethodPost, "/v1/providers/mitake/callbacks", strings.NewReader(`{"msgid":"r1"}`))
	js.Header.Set("Content-Type", "application/json")
	js.Header.Set(SecretHeader, testSecret)
	if _, err := p.ParseCallback(js); !errors.Is(err, provider.ErrPermanent) {
		t.Errorf("JSON: err = %v, want ErrPermanent", err)
	}

	big := callback(url.Values{"msgid": {strings.Repeat("x", maxCallbackBytes)}, "statuscode": {"4"}}, testSecret)
	if _, err := p.ParseCallback(big); !errors.Is(err, provider.ErrPermanent) {
		t.Errorf("oversized: err = %v, want ErrPermanent", err)
	}
}
