package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"example.com/sms-platform/internal/template"
)

// Templates is tenant-scoped template CRUD; every method takes the tenant from
// the request context first (AD-4).
type Templates interface {
	Create(tenantID, name, body string) (template.Template, error)
	Get(tenantID, id string) (template.Template, error)
	List(tenantID string) ([]template.Template, error)
	Delete(tenantID, id string) error
}

type templateRequest struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

type templateResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func toTemplateResponse(t template.Template) templateResponse {
	return templateResponse{ID: t.ID, Name: t.Name, Body: t.Body, CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339)}
}

// registerTemplates mounts /v1/templates. There is no update route (PUT).
func (s *server) registerTemplates(mux *http.ServeMux) {
	if s.deps.Templates == nil {
		panic("httpapi: Deps.Templates is required")
	}
	mux.Handle("POST /v1/templates", s.v1(s.handler(s.createTemplate)))
	mux.Handle("GET /v1/templates", s.v1(s.handler(s.listTemplates)))
	mux.Handle("GET /v1/templates/{id}", s.v1(s.handler(s.getTemplate)))
	mux.Handle("DELETE /v1/templates/{id}", s.v1(s.handler(s.deleteTemplate)))
}

func (s *server) createTemplate(w http.ResponseWriter, r *http.Request) error {
	var req templateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes)).Decode(&req); err != nil {
		return invalid("request body must be valid JSON")
	}
	if strings.TrimSpace(req.Name) == "" {
		return invalid("field 'name' is required")
	}
	if strings.TrimSpace(req.Body) == "" {
		return invalid("field 'body' is required")
	}
	t, err := s.deps.Templates.Create(tenantID(r.Context()), req.Name, req.Body)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, toTemplateResponse(t))
}

func (s *server) listTemplates(w http.ResponseWriter, r *http.Request) error {
	ts, err := s.deps.Templates.List(tenantID(r.Context()))
	if err != nil {
		return err
	}
	out := make([]templateResponse, len(ts))
	for i, t := range ts {
		out[i] = toTemplateResponse(t)
	}
	return writeJSON(w, http.StatusOK, map[string][]templateResponse{"templates": out})
}

func (s *server) getTemplate(w http.ResponseWriter, r *http.Request) error {
	t, err := s.deps.Templates.Get(tenantID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, toTemplateResponse(t))
}

func (s *server) deleteTemplate(w http.ResponseWriter, r *http.Request) error {
	if err := s.deps.Templates.Delete(tenantID(r.Context()), r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
