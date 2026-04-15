package httpapi

import (
	"database/sql"
	"net/http"
)

// NewHandler returns the root HTTP router for the API.
func NewHandler(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("POST /auth/register", registerHandler(db))

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
