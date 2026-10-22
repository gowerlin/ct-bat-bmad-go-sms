package httpapi

import (
	"net/http"
	"time"

	"example.com/sms-platform/internal/provider"
	"example.com/sms-platform/internal/provider/mock"
)

// Deps are the collaborators the handler needs. Every field is optional; a
// nil field falls back to the production default. Tests inject fakes from
// e2e/testutil so that time and the SMS provider are deterministic.
type Deps struct {
	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time
	// Provider sends messages. Defaults to the mock provider.
	Provider provider.Provider
}

func (d Deps) withDefaults() Deps {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Provider == nil {
		d.Provider = mock.New()
	}
	return d
}

// NewHandler builds the platform's HTTP handler. The route table mirrors
// docs/openapi.yaml; only GET /healthz is implemented in the skeleton.
func NewHandler(deps Deps) http.Handler {
	deps = deps.withDefaults()
	_ = deps // wired into routes by later work orders (T0001 onward)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	return mux
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
