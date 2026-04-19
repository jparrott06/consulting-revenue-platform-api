package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONBody_UnknownFieldReturns400(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"email":"a@b.com","extra":1}`))

	type payload struct {
		Email string `json:"email"`
	}
	ok := decodeJSONBody(req.Context(), rec, req, &payload{})
	if ok {
		t.Fatal("expected decode failure")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env["code"] != "validation_error" {
		t.Fatalf("code: %v", env["code"])
	}
}
