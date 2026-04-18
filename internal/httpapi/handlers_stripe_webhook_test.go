package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/stripe/stripe-go/v81/webhook"
)

func TestStripeWebhook_InvalidSignature(t *testing.T) {
	db, _ := newSQLMock(t)
	cfg := config.Config{StripeWebhookSecret: "whsec_testsecretvalueforwebhook123456"}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader([]byte(`{"id":"evt_x"}`)))
	req.Header.Set("Stripe-Signature", "t=1,v1=deadbeef")
	rec := httptest.NewRecorder()
	NewHandler(cfg, db).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStripeWebhook_ValidSignaturePersisted(t *testing.T) {
	raw := []byte(`{
  "id": "evt_test_webhook_1",
  "object": "event",
  "api_version": "2024-11-20.acacia",
  "created": 1700000000,
  "data": {"object": {"id": "pi_123"}},
  "livemode": false,
  "pending_webhooks": 0,
  "type": "payment_intent.succeeded"
}`)
	secret := "whsec_testsecretvalueforwebhook123456"
	sp := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   raw,
		Secret:    secret,
		Timestamp: time.Now().Add(-30 * time.Second),
	})

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg := config.Config{StripeWebhookSecret: secret}

	mock.ExpectExec(`INSERT INTO webhook_events`).
		WithArgs("evt_test_webhook_1", "payment_intent.succeeded", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(1, 1))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(sp.Payload))
	req.Header.Set("Stripe-Signature", sp.Header)
	rec := httptest.NewRecorder()
	NewHandler(cfg, db).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["inserted"] != true {
		t.Fatalf("expected inserted true, got %#v", out["inserted"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStripeWebhook_MissingSecret503(t *testing.T) {
	db, _ := newSQLMock(t)
	cfg := config.Config{}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader("{}"))
	req.Header.Set("Stripe-Signature", "t=1,v1=x")
	rec := httptest.NewRecorder()
	NewHandler(cfg, db).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
