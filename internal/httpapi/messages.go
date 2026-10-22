package httpapi

import (
	"time"

	"example.com/sms-platform/internal/store"
)

// messageResponse is the shared message DTO (getMessage and the list route).
type messageResponse struct {
	ID            string `json:"id"`
	To            string `json:"to"`
	Status        string `json:"status"`
	Attempts      int    `json:"attempts"`
	FailureReason string `json:"failure_reason,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func toMessageResponse(m store.Message) messageResponse {
	return messageResponse{
		ID:            m.ID,
		To:            m.To,
		Status:        string(m.Status),
		Attempts:      m.Attempts,
		FailureReason: m.FailureReason,
		CreatedAt:     m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     m.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
