package repo

import (
	"reflect"
	"testing"
)

func TestRedactAuditMetadata_RemovesSecrets(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"password":         "x",
		"nested":           map[string]any{"refresh_token": "tok", "ok": 1},
		"stripe_signature": "sig",
		"list":             []any{map[string]any{"api_key": "k"}},
	}
	got := RedactAuditMetadata(in)
	out, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if out["password"] != "[REDACTED]" {
		t.Fatalf("password: %#v", out["password"])
	}
	nested, _ := out["nested"].(map[string]any)
	if nested["refresh_token"] != "[REDACTED]" {
		t.Fatalf("nested refresh_token: %#v", nested["refresh_token"])
	}
	if nested["ok"] != 1 {
		t.Fatalf("nested ok: %#v", nested["ok"])
	}
	if out["stripe_signature"] != "[REDACTED]" {
		t.Fatalf("stripe_signature: %#v", out["stripe_signature"])
	}

	lst, _ := out["list"].([]any)
	item, _ := lst[0].(map[string]any)
	if item["api_key"] != "[REDACTED]" {
		t.Fatalf("api_key: %#v", item["api_key"])
	}
}

func TestRedactAuditMetadata_PreservesSafeFields(t *testing.T) {
	t.Parallel()

	in := map[string]any{"invoice_id": "abc", "amount_minor": int64(5)}
	got := RedactAuditMetadata(in)
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("unexpected change: %#v", got)
	}
}
