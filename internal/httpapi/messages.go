package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"example.com/sms-platform/internal/queue"
	"example.com/sms-platform/internal/store"
)

const maxRequestBytes = 1 << 20

type sendRequest struct {
	To   string `json:"to"`
	Body string `json:"body"`
}

type messageResponse struct {
	ID            string `json:"id"`
	To            string `json:"to"`
	Status        string `json:"status"`
	Attempts      int    `json:"attempts"`
	FailureReason string `json:"failure_reason,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func (s *server) sendMessage(w http.ResponseWriter, r *http.Request) error {
	var req sendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes)).Decode(&req); err != nil {
		return invalid("request body must be valid JSON")
	}
	if err := validateMessage(req.To, req.Body); err != nil {
		return err
	}
	id, err := s.deps.Queue.Submit(r.Context(), tenantID(r.Context()), queue.Draft{To: req.To, Body: req.Body})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusAccepted, map[string]string{"id": id, "status": string(store.StatusQueued)})
}

func (s *server) getMessage(w http.ResponseWriter, r *http.Request) error {
	m, err := s.deps.Store.Get(tenantID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, messageResponse{
		ID:            m.ID,
		To:            m.To,
		Status:        string(m.Status),
		Attempts:      m.Attempts,
		FailureReason: m.FailureReason,
		CreatedAt:     m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     m.UpdatedAt.UTC().Format(time.RFC3339),
	})
}
