package httpapi

import "net/http"

// NewHandler builds the platform's HTTP handler. The route table mirrors
// docs/openapi.yaml; each feature registers its own routes in its own file.
func NewHandler(deps Deps) http.Handler {
	if deps.Queue == nil || deps.Store == nil || deps.Tenants == nil {
		panic("httpapi: Deps.Queue, Deps.Store and Deps.Tenants are required")
	}
	s := &server{deps: deps}

	mux := http.NewServeMux()
	registerHealth(mux)
	s.registerMessagesSend(mux)
	s.registerMessagesGet(mux)
	s.registerMessagesList(mux)
	s.registerTemplates(mux)
	return envelopeFallbacks(mux)
}

type server struct{ deps Deps }

// v1 wraps a /v1 route in its middleware. The order is fixed (AD-10):
// authenticate, then rateLimit, then the handler, so an invalid key has no
// bucket and gets 401 directly.
func (s *server) v1(h http.Handler) http.Handler {
	return s.authenticate(s.rateLimit(h))
}
