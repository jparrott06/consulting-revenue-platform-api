package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
)

// APIError is the canonical error response envelope.
type APIError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	Details   map[string]any `json:"details,omitempty"`
}

func writeError(ctx context.Context, w http.ResponseWriter, status int, code, message string, details map[string]any) {
	writeJSON(w, status, APIError{
		Code:      code,
		Message:   message,
		RequestID: requestIDFromContext(ctx),
		Details:   details,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
