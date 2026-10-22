package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
	"example.com/sms-platform/internal/template"
)

// sendRequest carries exactly one of Body or TemplateID; Vars fill the
// template's placeholders and only take string values (a pointer so that a
// null value is rejected instead of becoming "").
type sendRequest struct {
	To         string             `json:"to"`
	Body       string             `json:"body"`
	TemplateID string             `json:"template_id"`
	Vars       map[string]*string `json:"vars"`
}

const errVarsShape = "field 'vars' must be an object of string values"

// templateVars converts Vars for template.Render, rejecting null values.
func (req sendRequest) templateVars() (map[string]string, error) {
	vars := make(map[string]string, len(req.Vars))
	for k, v := range req.Vars {
		if v == nil {
			return nil, invalid(errVarsShape)
		}
		vars[k] = *v
	}
	return vars, nil
}

func (s *server) registerMessagesSend(mux *http.ServeMux) {
	mux.Handle("POST /v1/messages", s.v1(s.handler(s.sendMessage)))
}

func (s *server) sendMessage(w http.ResponseWriter, r *http.Request) error {
	var req sendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes)).Decode(&req); err != nil {
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) && (te.Field == "vars" || strings.HasPrefix(te.Field, "vars.")) {
			return invalid(errVarsShape)
		}
		return invalid("request body must be valid JSON")
	}
	tid := tenantID(r.Context())
	body := req.Body
	switch {
	case req.Body != "" && req.TemplateID != "":
		return invalid("fields 'body' and 'template_id' are mutually exclusive")
	case req.Body == "" && req.TemplateID == "":
		return invalid("field 'body' or 'template_id' is required")
	case req.TemplateID != "":
		vars, err := req.templateVars()
		if err != nil {
			return err
		}
		// Render before Submit, so a failed render leaves no message (AD-16).
		t, err := s.deps.Templates.Get(tid, req.TemplateID)
		if err != nil {
			return err
		}
		if body, err = template.Render(t.Body, vars); err != nil {
			return err
		}
	}
	if err := validateMessage(req.To, body); err != nil {
		return err
	}
	id, err := s.deps.Queue.Submit(r.Context(), tid, queue.Draft{To: req.To, Body: body})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusAccepted, map[string]string{"id": id, "status": string(store.StatusQueued)})
}
