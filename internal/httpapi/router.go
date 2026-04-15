package httpapi

import (
	"encoding/json"
	"net/http"
)

// NewHandler returns the root HTTP router for the API.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)

	return chain(
		mux,
		requestIDMiddleware,
		recoveryMiddleware,
		timeoutMiddleware,
		loggingMiddleware,
	)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
