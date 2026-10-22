package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/provider/mitake"
	"example.com/sms-platform/internal/provider/mock"
)

// Environment keys that select and configure the provider (AD-18, D017,
// D019). Values are never logged; errors name keys only.
const (
	envProvider       = "PROVIDER"
	envProviderURL    = "PROVIDER_URL"
	envMitakeUsername = "MITAKE_USERNAME"
	envMitakePassword = "MITAKE_PASSWORD"
	envCallbackSecret = "PROVIDER_CALLBACK_SECRET"

	validProviders = "mock|mitake"

	providerHTTPTimeout = 10 * time.Second
)

// providerConfigError is a startup failure that is safe to log: it carries a
// class for error_class and names environment keys, never their values.
type providerConfigError struct {
	class string
	msg   string
}

func (e *providerConfigError) Error() string { return e.msg }

// newProvider picks the provider named by PROVIDER. There is no default:
// unset, empty or unknown values fail, as does PROVIDER=mitake with any of its
// settings missing (AD-11).
func newProvider(getenv func(string) string, now func() time.Time) (provider.Provider, error) {
	switch name := getenv(envProvider); name {
	case mock.Name:
		return mock.NewWithClock(now), nil
	case mitake.Name:
		return newMitake(getenv, now)
	case "":
		return nil, &providerConfigError{"provider_unset", fmt.Sprintf("%s is required; valid values: %s", envProvider, validProviders)}
	default:
		return nil, &providerConfigError{"provider_invalid", fmt.Sprintf("%s is not a valid provider; valid values: %s", envProvider, validProviders)}
	}
}

func newMitake(getenv func(string) string, now func() time.Time) (provider.Provider, error) {
	cfg := mitake.Config{
		BaseURL:        getenv(envProviderURL),
		Username:       getenv(envMitakeUsername),
		Password:       getenv(envMitakePassword),
		CallbackSecret: getenv(envCallbackSecret),
	}
	var missing []string
	for _, kv := range []struct{ key, val string }{
		{envProviderURL, cfg.BaseURL},
		{envMitakeUsername, cfg.Username},
		{envMitakePassword, cfg.Password},
		{envCallbackSecret, cfg.CallbackSecret},
	} {
		if kv.val == "" {
			missing = append(missing, kv.key)
		}
	}
	if len(missing) > 0 {
		return nil, &providerConfigError{"provider_config_missing", fmt.Sprintf("%s=mitake requires %v", envProvider, missing)}
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, &providerConfigError{"provider_config_invalid", fmt.Sprintf("%s must be an absolute http(s) URL", envProviderURL)}
	}
	p, err := mitake.New(cfg, newProviderClient(u.Host), now)
	if err != nil {
		return nil, &providerConfigError{"provider_config_invalid", "mitake configuration rejected"}
	}
	return p, nil
}

// logProviderError reports a startup failure without echoing any value.
func logProviderError(log *slog.Logger, err error) {
	class := "provider_config"
	var ce *providerConfigError
	if errors.As(err, &ce) {
		class = ce.class
	}
	log.Error("provider configuration invalid", "error_class", class, "detail", err.Error(), "valid_providers", validProviders)
}

// errHostNotAllowed is returned for a request outside the allow-list (AD-9).
var errHostNotAllowed = errors.New("outbound host not in allow-list")

// allowListTransport only lets requests reach the listed hosts (AD-9). The
// list holds the provider address; tenant webhook URLs join it later (T0009).
type allowListTransport struct {
	allowed map[string]bool
	next    http.RoundTripper
}

func (t allowListTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if !t.allowed[r.URL.Host] {
		if r.Body != nil {
			_ = r.Body.Close()
		}
		return nil, errHostNotAllowed
	}
	return t.next.RoundTrip(r)
}

// newProviderClient builds the only HTTP client the provider gets (AD-9):
// allow-listed hosts, a bounded timeout and no redirects.
func newProviderClient(hosts ...string) *http.Client {
	allowed := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		allowed[h] = true
	}
	c := new(http.Client)
	c.Transport = allowListTransport{allowed: allowed, next: http.DefaultTransport.(*http.Transport).Clone()}
	c.Timeout = providerHTTPTimeout
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}
