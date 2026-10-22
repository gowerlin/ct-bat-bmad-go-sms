package e2e

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"example.com/sms-platform/e2e/testutil"
	"example.com/sms-platform/internal/tenant"
)

// retryAfter asserts a 429 rate_limited envelope and returns its Retry-After,
// which must be a positive integer number of seconds.
func retryAfter(t *testing.T, resp *http.Response, body []byte) int {
	t.Helper()
	wantEnvelope(t, resp, body, http.StatusTooManyRequests, "rate_limited")
	raw := resp.Header.Get("Retry-After")
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		t.Fatalf("Retry-After = %q, want a positive integer", raw)
	}
	return n
}

func TestE2E_RateLimit_Returns429WithRetryAfter(t *testing.T) {
	env := testutil.NewTestServer(t)
	// 3 per minute: a full bucket of 3, one token back every 20s.
	env.Tenants.AddTenant(tenant.Tenant{ID: testutil.TenantA, RateLimitPerMin: 3})
	const keyA2 = "test-key-tenant-a-2"
	env.Tenants.AddKey(testutil.TenantA, keyA2)
	const payload = `{"to":"+886900000001","body":"hi"}`
	list := func(key string) (*http.Response, []byte) {
		return env.Do(t, http.MethodGet, "/v1/templates", key, nil)
	}

	for i := 1; i <= 3; i++ {
		if resp, body := list(testutil.KeyA); resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status = %d; body = %s", i, resp.StatusCode, body)
		}
	}

	resp, body := list(testutil.KeyA)
	if got := retryAfter(t, resp, body); got != 20 {
		t.Fatalf("Retry-After = %d, want 20 (ceil of 20s to the next token)", got)
	}

	t.Run("rejected send creates no message", func(t *testing.T) {
		before := env.Store.Len()
		resp, body := env.Do(t, http.MethodPost, "/v1/messages", testutil.KeyA, strings.NewReader(payload))
		retryAfter(t, resp, body)
		if env.Store.Len() != before {
			t.Fatalf("store grew from %d to %d", before, env.Store.Len())
		}
	})
	t.Run("other keys have their own buckets", func(t *testing.T) {
		if resp, body := list(testutil.KeyB); resp.StatusCode != http.StatusOK {
			t.Fatalf("tenant B: status = %d; body = %s", resp.StatusCode, body)
		}
		if resp, body := list(keyA2); resp.StatusCode != http.StatusOK {
			t.Fatalf("tenant A second key: status = %d; body = %s", resp.StatusCode, body)
		}
	})
	t.Run("healthz is not limited", func(t *testing.T) {
		if resp, body := env.Get(t, "/healthz"); resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d; body = %s", resp.StatusCode, body)
		}
	})
	t.Run("invalid key gets 401 not 429", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, body := list("test-key-unknown")
			wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
		}
		resp, body := list("")
		wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
	})

	// Halfway to the next token: still limited, with the remaining wait.
	env.Clock.Advance(10 * time.Second)
	resp, body = list(testutil.KeyA)
	if got := retryAfter(t, resp, body); got != 10 {
		t.Fatalf("Retry-After after 10s = %d, want 10", got)
	}

	// Waiting out Retry-After lets exactly one request through.
	env.Clock.Advance(10 * time.Second)
	if resp, body := list(testutil.KeyA); resp.StatusCode != http.StatusOK {
		t.Fatalf("after Retry-After: status = %d; body = %s", resp.StatusCode, body)
	}
	resp, body = list(testutil.KeyA)
	retryAfter(t, resp, body)
}
