package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"example.com/sms-platform/e2e/testutil"
)

func TestE2E_Tenant_IsolationReturns404(t *testing.T) {
	env := testutil.NewTestServer(t)

	resp, body := env.Do(t, http.MethodPost, "/v1/messages", testutil.KeyA,
		strings.NewReader(`{"to":"+886900000001","body":"hi"}`))
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("A send: %d %s", resp.StatusCode, body)
	}
	var sent struct{ ID string }
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatal(err)
	}
	path := "/v1/messages/" + sent.ID

	resp, body = env.Do(t, http.MethodGet, path, testutil.KeyB, nil)
	wantEnvelope(t, resp, body, http.StatusNotFound, "not_found")
	if strings.Contains(string(body), sent.ID) {
		t.Fatalf("404 body reveals the message id: %s", body)
	}

	resp, body = env.Do(t, http.MethodGet, path, testutil.KeyA, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("A own GET: %d %s", resp.StatusCode, body)
	}
}
