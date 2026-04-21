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

func stripeWebhookTestSecret() string {
	return "whsec_testsecretvalueforwebhook123456"
}

func expectStripeWebhookInserted(mock sqlmock.Sqlmock, eventID, eventType string, rowsAffected int64) {
	mock.ExpectExec(`INSERT INTO webhook_events`).
		WithArgs(eventID, eventType, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, rowsAffected))
	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestStripeWebhook_CheckoutSessionCompleted_Signed(t *testing.T) {
	raw := []byte(`{
  "id": "evt_checkout_session_completed_1",
  "object": "event",
  "api_version": "2024-11-20.acacia",
  "created": 1700000001,
  "data": {"object": {"id": "cs_test_123", "object": "checkout.session"}},
  "livemode": false,
  "pending_webhooks": 0,
  "type": "checkout.session.completed"
}`)
	secret := stripeWebhookTestSecret()
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
	expectStripeWebhookInserted(mock, "evt_checkout_session_completed_1", "checkout.session.completed", 1)

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

func TestStripeWebhook_PaymentIntentPaymentFailed_Signed(t *testing.T) {
	raw := []byte(`{
  "id": "evt_payment_intent_failed_1",
  "object": "event",
  "api_version": "2024-11-20.acacia",
  "created": 1700000002,
  "data": {"object": {"id": "pi_failed_1", "object": "payment_intent", "status": "requires_payment_method"}},
  "livemode": false,
  "pending_webhooks": 0,
  "type": "payment_intent.payment_failed"
}`)
	secret := stripeWebhookTestSecret()
	sp := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   raw,
		Secret:    secret,
		Timestamp: time.Now().Add(-60 * time.Second),
	})

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg := config.Config{StripeWebhookSecret: secret}
	expectStripeWebhookInserted(mock, "evt_payment_intent_failed_1", "payment_intent.payment_failed", 1)

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

func TestStripeWebhook_ChargeRefunded_Signed(t *testing.T) {
	raw := []byte(`{
  "id": "evt_charge_refunded_1",
  "object": "event",
  "api_version": "2024-11-20.acacia",
  "created": 1700000003,
  "data": {"object": {"id": "ch_refund_1", "object": "charge", "refunded": true}},
  "livemode": false,
  "pending_webhooks": 0,
  "type": "charge.refunded"
}`)
	secret := stripeWebhookTestSecret()
	sp := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   raw,
		Secret:    secret,
		Timestamp: time.Now().Add(-90 * time.Second),
	})

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg := config.Config{StripeWebhookSecret: secret}
	expectStripeWebhookInserted(mock, "evt_charge_refunded_1", "charge.refunded", 1)

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

func TestStripeWebhook_ReplaySameEventIdempotent(t *testing.T) {
	raw := []byte(`{
  "id": "evt_replay_idempotent_1",
  "object": "event",
  "api_version": "2024-11-20.acacia",
  "created": 1700000004,
  "data": {"object": {"id": "pi_replay"}},
  "livemode": false,
  "pending_webhooks": 0,
  "type": "payment_intent.succeeded"
}`)
	secret := stripeWebhookTestSecret()
	sp := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   raw,
		Secret:    secret,
		Timestamp: time.Now().Add(-45 * time.Second),
	})

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg := config.Config{StripeWebhookSecret: secret}

	mock.ExpectExec(`INSERT INTO webhook_events`).
		WithArgs("evt_replay_idempotent_1", "payment_intent.succeeded", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO webhook_events`).
		WithArgs("evt_replay_idempotent_1", "payment_intent.succeeded", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(1, 1))

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(sp.Payload))
		req.Header.Set("Stripe-Signature", sp.Header)
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		return rec
	}

	rec1 := post()
	if rec1.Code != http.StatusOK {
		t.Fatalf("first: expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var out1 map[string]any
	if err := json.NewDecoder(rec1.Body).Decode(&out1); err != nil {
		t.Fatal(err)
	}
	if out1["inserted"] != true {
		t.Fatalf("first: expected inserted true, got %#v", out1["inserted"])
	}

	rec2 := post()
	if rec2.Code != http.StatusOK {
		t.Fatalf("second: expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var out2 map[string]any
	if err := json.NewDecoder(rec2.Body).Decode(&out2); err != nil {
		t.Fatal(err)
	}
	if out2["inserted"] != false {
		t.Fatalf("second: expected inserted false (duplicate), got %#v", out2["inserted"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
