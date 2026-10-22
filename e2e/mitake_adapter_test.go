package e2e

// stories.yaml id 12（T0010；D014 send、D016 失敗類別、D017 回呼格式）。
// 假對端模擬三竹「風格」的供應商，不是真實規格：帳號密碼與密鑰都是假的，
// 全程只連 httptest.Server，不連任何真實端點。

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/provider/mitake"
)

const (
	fakeMitakeUser   = "fake-user"
	fakeMitakePass   = "fake-password-do-not-log"
	fakeMitakeSecret = "fake-callback-secret"
)

var demoNumber = regexp.MustCompile(`^\+8869000000\d{2}$`)

// fakeMitake is the supplier side. It validates every send request itself and
// answers according to the clientid, so each case exercises the real wire
// format rather than a canned string.
type fakeMitake struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	accepted []url.Values
	rejected []string
}

// replies maps clientid to the supplier's answer (status code, body).
var mitakeReplies = map[string]struct {
	status int
	body   string
}{
	"msg_ok":        {http.StatusOK, "[1]\r\nmsgid=7000000001\r\nstatuscode=1\r\nAccountPoint=99\r\n"},
	"msg_busy":      {http.StatusOK, "[1]\r\nstatuscode=t\r\n"},
	"msg_down":      {http.StatusServiceUnavailable, ""},
	"msg_rejected":  {http.StatusOK, "[1]\r\nstatuscode=e\r\n"},
	"msg_forbidden": {http.StatusForbidden, ""},
}

func newFakeMitake(t *testing.T) *fakeMitake {
	f := &fakeMitake{t: t}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serveSend))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeMitake) serveSend(w http.ResponseWriter, r *http.Request) {
	if problem := validateSend(r); problem != "" {
		f.mu.Lock()
		f.rejected = append(f.rejected, problem)
		f.mu.Unlock()
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.accepted = append(f.accepted, r.PostForm)
	f.mu.Unlock()
	reply, ok := mitakeReplies[r.PostForm.Get("clientid")]
	if !ok {
		http.Error(w, "unknown scenario", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(reply.status)
	_, _ = w.Write([]byte(reply.body))
}

// validateSend checks the request the adapter put on the wire (D014).
func validateSend(r *http.Request) string {
	if r.Method != http.MethodPost {
		return "method " + r.Method
	}
	if r.URL.Path != mitake.SendPath {
		return "path " + r.URL.Path
	}
	if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		return "content type " + ct
	}
	if err := r.ParseForm(); err != nil {
		return "form: " + err.Error()
	}
	if len(r.URL.Query()) != 0 {
		return "fields must be in the body, not the query"
	}
	f := r.PostForm
	for _, k := range []string{"username", "password", "dstaddr", "smbody", "CharsetURL", "clientid"} {
		if f.Get(k) == "" {
			return "missing field " + k
		}
	}
	switch {
	case f.Get("username") != fakeMitakeUser || f.Get("password") != fakeMitakePass:
		return "wrong credentials"
	case f.Get("CharsetURL") != "UTF-8":
		return "CharsetURL " + f.Get("CharsetURL")
	case !demoNumber.MatchString(f.Get("dstaddr")):
		return "dstaddr is not a +8869000000xx demo number"
	}
	return ""
}

// deliverCallback is the supplier posting a delivery report to the platform.
func (f *fakeMitake) deliverCallback(target string, form url.Values, secret string) *http.Response {
	f.t.Helper()
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if secret != "" {
		req.Header.Set(mitake.SecretHeader, secret)
	}
	resp, err := f.srv.Client().Do(req)
	if err != nil {
		f.t.Fatalf("deliver callback: %v", err)
	}
	return resp
}

func TestE2E_MitakeAdapter_ContractAgainstFakeServer(t *testing.T) {
	fake := newFakeMitake(t)
	now := func() time.Time { return time.Date(2026, 10, 23, 2, 0, 0, 0, time.UTC) }
	p, err := mitake.New(mitake.Config{
		BaseURL: fake.srv.URL, Username: fakeMitakeUser, Password: fakeMitakePass, CallbackSecret: fakeMitakeSecret,
	}, fake.srv.Client(), now)
	if err != nil {
		t.Fatalf("mitake.New: %v", err)
	}
	send := func(id string) (provider.ProviderReceipt, error) {
		return p.Send(context.Background(), provider.OutboundMessage{
			ID: id, TenantID: "tenant_a", To: "+886900000042", Body: "驗證碼 0000（測試）",
		})
	}
	noLeak := func(t *testing.T, err error) {
		t.Helper()
		for _, s := range []string{fakeMitakePass, "+886900000042", "驗證碼"} {
			if strings.Contains(err.Error(), s) {
				t.Fatalf("error leaks %q: %v", s, err)
			}
		}
	}

	t.Run("success", func(t *testing.T) {
		r, err := send("msg_ok")
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if r.ProviderRef != "7000000001" {
			t.Fatalf("ProviderRef = %q, want the msgid from the reply", r.ProviderRef)
		}
		if !strings.HasPrefix(r.Raw, "[1]") || !r.AcceptedAt.Equal(now()) {
			t.Fatalf("receipt = %+v", r)
		}
		fake.mu.Lock()
		last := fake.accepted[len(fake.accepted)-1]
		fake.mu.Unlock()
		if last.Get("smbody") != "驗證碼 0000（測試）" || last.Get("dstaddr") != "+886900000042" {
			t.Fatalf("fake received %v", last)
		}
	})

	for _, tc := range []struct{ name, id string }{
		{"transient statuscode", "msg_busy"},
		{"transient http 503", "msg_down"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := send(tc.id)
			if !errors.Is(err, provider.ErrTransient) || errors.Is(err, provider.ErrPermanent) {
				t.Fatalf("err = %v, want ErrTransient only", err)
			}
			noLeak(t, err)
		})
	}

	for _, tc := range []struct{ name, id string }{
		{"permanent statuscode", "msg_rejected"},
		{"permanent http 403", "msg_forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := send(tc.id)
			if !errors.Is(err, provider.ErrPermanent) || errors.Is(err, provider.ErrTransient) {
				t.Fatalf("err = %v, want ErrPermanent only", err)
			}
			noLeak(t, err)
		})
	}

	t.Run("callback", func(t *testing.T) {
		type parsed struct {
			rep provider.DeliveryReport
			err error
		}
		got := make(chan parsed, 1)
		platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rep, err := p.ParseCallback(r)
			got <- parsed{rep, err}
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(platform.Close)
		target := platform.URL + "/v1/providers/mitake/callbacks"

		cases := []struct {
			name     string
			form     url.Values
			secret   string
			want     provider.Status
			wantErr  error
			received time.Time
		}{
			{"delivered", url.Values{"msgid": {"7000000001"}, "statuscode": {"4"}, "donetime": {"20261023101500"}}, fakeMitakeSecret,
				provider.StatusDelivered, nil, time.Date(2026, 10, 23, 2, 15, 0, 0, time.UTC)},
			{"undelivered", url.Values{"msgid": {"7000000001"}, "statuscode": {"5"}}, fakeMitakeSecret,
				provider.StatusUndelivered, nil, now()},
			{"sent is not a callback status", url.Values{"msgid": {"7000000001"}, "statuscode": {"1"}}, fakeMitakeSecret,
				"", provider.ErrPermanent, time.Time{}},
			{"wrong secret", url.Values{"msgid": {"7000000001"}, "statuscode": {"4"}}, "not-the-secret",
				"", provider.ErrUnauthorized, time.Time{}},
			{"missing secret", url.Values{"msgid": {"7000000001"}, "statuscode": {"4"}}, "",
				"", provider.ErrUnauthorized, time.Time{}},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				resp := fake.deliverCallback(target, c.form, c.secret)
				_ = resp.Body.Close()
				res := <-got
				if c.wantErr != nil {
					if !errors.Is(res.err, c.wantErr) {
						t.Fatalf("err = %v, want %v", res.err, c.wantErr)
					}
					return
				}
				if res.err != nil {
					t.Fatalf("ParseCallback: %v", res.err)
				}
				if res.rep.ProviderRef != "7000000001" || res.rep.Status != c.want || !res.rep.ReceivedAt.Equal(c.received) {
					t.Fatalf("report = %+v, want ref 7000000001, %s at %v", res.rep, c.want, c.received)
				}
				if strings.Contains(res.rep.Raw, fakeMitakeSecret) {
					t.Fatalf("Raw contains the secret: %q", res.rep.Raw)
				}
			})
		}
	})

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.rejected) != 0 {
		t.Fatalf("fake rejected malformed requests: %v", fake.rejected)
	}
	if len(fake.accepted) != 5 {
		t.Fatalf("fake accepted %d send requests, want 5", len(fake.accepted))
	}
}
