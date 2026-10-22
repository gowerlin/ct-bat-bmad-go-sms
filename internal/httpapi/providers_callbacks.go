package httpapi

import (
	"errors"
	"net/http"

	"example.com/sms-platform/internal/webhook"
)

// maxCallbackBytes bounds a supplier callback body.
const maxCallbackBytes = 64 << 10

// Callbacks applies a supplier callback for provider name; *webhook.Inbound
// satisfies it. The provider's ParseCallback runs inside, so httpapi still
// never holds a Provider (AD-1, AD-7).
type Callbacks interface {
	Receive(name string, r *http.Request) (webhook.Outcome, error)
}

// registerProviderCallbacks adds POST /v1/providers/{name}/callbacks. It is
// outside s.v1: suppliers carry no API key and are not rate-limited (AD-7).
// Without Deps.Callbacks the route is absent and every path is 404.
func (s *server) registerProviderCallbacks(mux *http.ServeMux) {
	if s.deps.Callbacks == nil {
		return
	}
	mux.Handle("POST /v1/providers/{name}/callbacks", s.handler(s.providerCallback))
}

// providerCallback answers 204 when the callback moved the message and 200
// when it was a repeat or the message was already terminal (D015).
func (s *server) providerCallback(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxCallbackBytes)
	out, err := s.deps.Callbacks.Receive(r.PathValue("name"), r)
	if err != nil {
		return err
	}
	if out == webhook.Applied {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	w.WriteHeader(http.StatusOK)
	return nil
}

// callbackError maps webhook sentinel errors to apiError. It is listed in
// featureErrorMappers so writeError does not change for them.
func callbackError(err error) (*apiError, bool) {
	switch {
	case errors.Is(err, webhook.ErrUnknownProvider), errors.Is(err, webhook.ErrUnknownMessage):
		return &apiError{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}, true
	case errors.Is(err, webhook.ErrUnauthorized):
		return &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "callback verification failed"}, true
	case errors.Is(err, webhook.ErrInvalidReport):
		return &apiError{
			Status: http.StatusBadRequest, Code: "invalid_request",
			Message: "callback must report delivered or undelivered for a provider reference",
		}, true
	}
	return nil, false
}
