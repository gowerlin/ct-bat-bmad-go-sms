package httpapi

import "net/http"

func (s *server) registerMessagesGet(mux *http.ServeMux) {
	mux.Handle("GET /v1/messages/{id}", s.v1(s.handler(s.getMessage)))
}

func (s *server) getMessage(w http.ResponseWriter, r *http.Request) error {
	m, err := s.deps.Store.Get(tenantID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, toMessageResponse(m))
}
