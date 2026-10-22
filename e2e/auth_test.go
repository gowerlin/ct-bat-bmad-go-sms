package e2e

import (
	"net/http"
	"strings"
	"testing"

	"example.com/sms-platform/e2e/testutil"
)

func TestE2E_Auth_MissingKey401(t *testing.T) {
	env := testutil.NewTestServer(t)
	env.Tenants.AddKey(testutil.TenantA, "test-key-disabled")
	env.Tenants.DisableKey("test-key-disabled")
	const payload = `{"to":"+886900000001","body":"hi"}`

	t.Run("missing key", func(t *testing.T) {
		resp, body := env.Do(t, http.MethodPost, "/v1/messages", "", strings.NewReader(payload))
		wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
	})
	t.Run("missing key on GET", func(t *testing.T) {
		resp, body := env.Do(t, http.MethodGet, "/v1/messages/msg_x", "", nil)
		wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
	})
	t.Run("invalid key", func(t *testing.T) {
		resp, body := env.Do(t, http.MethodPost, "/v1/messages", "test-key-unknown", strings.NewReader(payload))
		wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
	})
	t.Run("disabled key", func(t *testing.T) {
		resp, body := env.Do(t, http.MethodPost, "/v1/messages", "test-key-disabled", strings.NewReader(payload))
		wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
	})
	t.Run("key in query string counts as missing", func(t *testing.T) {
		resp, body := env.Do(t, http.MethodPost, "/v1/messages?api_key="+testutil.KeyA, "", strings.NewReader(payload))
		wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
		resp, body = env.Do(t, http.MethodGet, "/v1/messages/msg_x?api_key="+testutil.KeyA, "", nil)
		wantEnvelope(t, resp, body, http.StatusUnauthorized, "unauthorized")
	})
	t.Run("rejected requests store nothing", func(t *testing.T) {
		if env.Store.Len() != 0 {
			t.Fatalf("store has %d messages after only unauthorized requests", env.Store.Len())
		}
	})
	t.Run("error never echoes the key", func(t *testing.T) {
		_, body := env.Do(t, http.MethodPost, "/v1/messages", "test-key-unknown", strings.NewReader(payload))
		if strings.Contains(string(body), "test-key-unknown") {
			t.Fatalf("body leaks key: %s", body)
		}
	})
	t.Run("valid key passes", func(t *testing.T) {
		resp, body := env.Do(t, http.MethodPost, "/v1/messages", testutil.KeyA, strings.NewReader(payload))
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("status = %d; body = %s", resp.StatusCode, body)
		}
	})
	t.Run("healthz needs no key", func(t *testing.T) {
		resp, body := env.Do(t, http.MethodGet, "/healthz", "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d; body = %s", resp.StatusCode, body)
		}
	})
}
