package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"example.com/sms-platform/e2e/testutil"
)

// TestE2E_Healthz is the first acceptance gate (COURSE-PLAN § 5.1 #8,
// T0001): the real handler, served over HTTP, answers GET /healthz with
// 200 and {"status":"ok"}.
func TestE2E_Healthz(t *testing.T) {
	env := testutil.NewTestServer(t)

	resp, body := env.Get(t, "/healthz")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz: status = %d, want %d; body = %s", resp.StatusCode, http.StatusOK, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("GET /healthz: Content-Type = %q, want application/json", ct)
	}
	var got struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("GET /healthz: body %q is not JSON: %v", body, err)
	}
	if got.Status != "ok" {
		t.Fatalf("GET /healthz: status field = %q, want %q", got.Status, "ok")
	}
}
