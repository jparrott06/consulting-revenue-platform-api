package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// decodeJSONBody decodes a single JSON object from r.Body into dst.
// Unknown fields, syntax errors, and oversized bodies map to consistent API errors.
func decodeJSONBody(ctx context.Context, w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(ctx, w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds maximum allowed size", map[string]any{
				"max_bytes": mbe.Limit,
			})
			return false
		}
		if errors.Is(err, io.EOF) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "empty JSON body", nil)
			return false
		}
		var syn *json.SyntaxError
		if errors.As(err, &syn) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
			return false
		}
		var ufe *json.UnmarshalTypeError
		if errors.As(err, &ufe) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
			return false
		}
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return false
		}
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return false
	}
	if dec.More() {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "trailing JSON content is not allowed", nil)
		return false
	}
	return true
}
