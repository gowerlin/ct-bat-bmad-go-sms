package httpapi

import "net/http"

// registerHealth mounts /healthz outside v1: no authentication, no rate limit.
func registerHealth(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", handleHealthz)
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
