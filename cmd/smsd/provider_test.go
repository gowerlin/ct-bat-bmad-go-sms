package main

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Fake values only (AGENTS.md Policy).
const (
	fakePassword = "fake-password-do-not-log"
	fakeSecret   = "fake-callback-secret"
)

func fixedNow() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func mitakeEnv() map[string]string {
	return map[string]string{
		envProvider:       "mitake",
		envProviderURL:    "http://127.0.0.1:9",
		envMitakeUsername: "fake-user",
		envMitakePassword: fakePassword,
		envCallbackSecret: fakeSecret,
	}
}

func TestNewProvider_Valid(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{"mock", map[string]string{envProvider: "mock"}, "mock"},
		{"mitake complete", mitakeEnv(), "mitake"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := newProvider(envOf(tc.env), fixedNow)
			if err != nil {
				t.Fatalf("newProvider: %v", err)
			}
			if p.Name() != tc.want {
				t.Fatalf("Name() = %q, want %q", p.Name(), tc.want)
			}
		})
	}
}

func TestNewProvider_FailsWithoutValidConfig(t *testing.T) {
	without := func(key string) map[string]string {
		m := mitakeEnv()
		delete(m, key)
		return m
	}
	badURL := mitakeEnv()
	badURL[envProviderURL] = "not a url"
	for _, tc := range []struct {
		name  string
		env   map[string]string
		class string
		names string // key the message must name
	}{
		{"unset", map[string]string{}, "provider_unset", envProvider},
		{"empty", map[string]string{envProvider: ""}, "provider_unset", envProvider},
		{"unknown", map[string]string{envProvider: "twilio"}, "provider_invalid", envProvider},
		{"wrong case", map[string]string{envProvider: "Mock"}, "provider_invalid", envProvider},
		{"mitake without URL", without(envProviderURL), "provider_config_missing", envProviderURL},
		{"mitake without username", without(envMitakeUsername), "provider_config_missing", envMitakeUsername},
		{"mitake without password", without(envMitakePassword), "provider_config_missing", envMitakePassword},
		{"mitake without callback secret", without(envCallbackSecret), "provider_config_missing", envCallbackSecret},
		{"mitake bad URL", badURL, "provider_config_invalid", envProviderURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := newProvider(envOf(tc.env), fixedNow)
			if err == nil {
				t.Fatalf("newProvider accepted it: %v", p.Name())
			}
			var ce *providerConfigError
			if !errors.As(err, &ce) || ce.class != tc.class {
				t.Fatalf("err = %#v, want class %q", err, tc.class)
			}
			if !strings.Contains(err.Error(), tc.names) {
				t.Fatalf("err = %q, want it to name %s", err, tc.names)
			}
			for _, secret := range []string{fakePassword, fakeSecret, "twilio"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("err echoes a value: %q", err)
				}
			}
		})
	}
}

func TestLogProviderError_ListsValidValuesWithoutSecrets(t *testing.T) {
	env := mitakeEnv()
	delete(env, envCallbackSecret)
	_, err := newProvider(envOf(env), fixedNow)
	var buf bytes.Buffer
	logProviderError(slog.New(slog.NewJSONHandler(&buf, nil)), err)
	out := buf.String()
	for _, want := range []string{`"level":"ERROR"`, `"error_class":"provider_config_missing"`, `"valid_providers":"mock|mitake"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log %s lacks %s", out, want)
		}
	}
	if strings.Contains(out, fakePassword) {
		t.Fatalf("log contains the password: %s", out)
	}
}

func TestProviderClient_AllowListOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)

	resp, err := newProviderClient(u.Host).Get(srv.URL)
	if err != nil {
		t.Fatalf("allowed host: %v", err)
	}
	_ = resp.Body.Close()

	if _, err := newProviderClient("127.0.0.1:1").Get(srv.URL); !errors.Is(err, errHostNotAllowed) {
		t.Fatalf("other host: err = %v, want errHostNotAllowed", err)
	}
}

func TestProviderClient_DoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://203.0.113.1/elsewhere", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	resp, err := newProviderClient(u.Host).Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want the redirect itself", resp.StatusCode)
	}
}
