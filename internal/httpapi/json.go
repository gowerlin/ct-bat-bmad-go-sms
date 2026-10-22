package httpapi

import (
	"encoding/json"
	"net/http"
)

const maxRequestBytes = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}
