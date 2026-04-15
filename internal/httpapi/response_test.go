package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorIncludesRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "req-123")
	rec := httptest.NewRecorder()

	writeError(ctx, rec, http.StatusBadRequest, "validation_error", "invalid payload", map[string]any{"field": "email"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var body APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Code != "validation_error" {
		t.Fatalf("unexpected code: %s", body.Code)
	}
	if body.RequestID != "req-123" {
		t.Fatalf("expected request id req-123, got %q", body.RequestID)
	}
}
