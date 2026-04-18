package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestApplyStripeRefund_PartialPaidInvoice(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	invID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	payID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	refundID := "re_test_1"
	piID := "pi_test_1"

	mock.ExpectBegin()

	lookupRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_x", nil, piID, "https://x", int64(5000), "USD", "k", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments WHERE stripe_payment_intent_id`).WithArgs(piID).WillReturnRows(lookupRows)

	invRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_number", "status", "currency", "subtotal_minor", "tax_minor", "total_minor",
		"issued_at", "due_at", "created_at", "updated_at",
	}).AddRow(invID, orgID, int64(1), "paid", "USD", int64(5000), int64(0), int64(5000),
		sql.NullTime{Valid: true}, sql.NullTime{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM invoices`).WithArgs(invID).WillReturnRows(invRows)

	payRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_x", nil, piID, "https://x", int64(5000), "USD", "k", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments`).WithArgs(orgID, invID).WillReturnRows(payRows)

	ledgerRows := sqlmock.NewRows([]string{"id"}).AddRow(uuid.MustParse("44444444-4444-4444-4444-444444444444"))
	mock.ExpectQuery(`INSERT INTO stripe_refund_events`).WillReturnRows(ledgerRows)

	mock.ExpectExec(`UPDATE payments`).WithArgs(int64(2000), payID).WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	in := StripeRefundInput{
		StripeRefundID:        refundID,
		StripePaymentIntentID: piID,
		AmountMinor:           2000,
		Currency:              "usd",
	}
	if err := ApplyStripeRefund(ctx, tx, in); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyStripeRefund_IdempotentReplay(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	invID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	orgID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	payID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	refundID := "re_dup"
	piID := "pi_dup"

	mock.ExpectBegin()

	lookupRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_y", nil, piID, "https://y", int64(100), "USD", "k2", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments WHERE stripe_payment_intent_id`).WithArgs(piID).WillReturnRows(lookupRows)

	invRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_number", "status", "currency", "subtotal_minor", "tax_minor", "total_minor",
		"issued_at", "due_at", "created_at", "updated_at",
	}).AddRow(invID, orgID, int64(2), "issued", "USD", int64(100), int64(0), int64(100),
		sql.NullTime{Valid: true}, sql.NullTime{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM invoices`).WithArgs(invID).WillReturnRows(invRows)

	payRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_y", nil, piID, "https://y", int64(100), "USD", "k2", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments`).WithArgs(orgID, invID).WillReturnRows(payRows)

	mock.ExpectQuery(`INSERT INTO stripe_refund_events`).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	in := StripeRefundInput{StripeRefundID: refundID, StripePaymentIntentID: piID, AmountMinor: 100, Currency: "usd"}
	if err := ApplyStripeRefund(ctx, tx, in); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyStripeRefund_FullRefundReopensPaidInvoice(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	invID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	orgID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	payID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	refundID := "re_full"
	piID := "pi_full"

	mock.ExpectBegin()

	lookupRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_f", nil, piID, "https://f", int64(5000), "USD", "kf", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments WHERE stripe_payment_intent_id`).WithArgs(piID).WillReturnRows(lookupRows)

	invRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_number", "status", "currency", "subtotal_minor", "tax_minor", "total_minor",
		"issued_at", "due_at", "created_at", "updated_at",
	}).AddRow(invID, orgID, int64(9), "paid", "USD", int64(5000), int64(0), int64(5000),
		sql.NullTime{Valid: true}, sql.NullTime{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM invoices`).WithArgs(invID).WillReturnRows(invRows)

	payRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_f", nil, piID, "https://f", int64(5000), "USD", "kf", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments`).WithArgs(orgID, invID).WillReturnRows(payRows)

	ledgerRows := sqlmock.NewRows([]string{"id"}).AddRow(uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"))
	mock.ExpectQuery(`INSERT INTO stripe_refund_events`).WillReturnRows(ledgerRows)

	mock.ExpectExec(`UPDATE payments`).WithArgs(int64(5000), payID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE invoices`).WithArgs(invID, orgID).WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	in := StripeRefundInput{
		StripeRefundID:        refundID,
		StripePaymentIntentID: piID,
		AmountMinor:           5000,
		Currency:              "usd",
	}
	if err := ApplyStripeRefund(ctx, tx, in); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecordStripePaymentFailure_UpdatesRow(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	invID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	orgID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	payID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	piID := "pi_fail_1"

	mock.ExpectBegin()

	lookupRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_z", nil, piID, "https://z", int64(200), "USD", "k3", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments WHERE stripe_payment_intent_id`).WithArgs(piID).WillReturnRows(lookupRows)

	invRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_number", "status", "currency", "subtotal_minor", "tax_minor", "total_minor",
		"issued_at", "due_at", "created_at", "updated_at",
	}).AddRow(invID, orgID, int64(3), "issued", "USD", int64(200), int64(0), int64(200),
		sql.NullTime{Valid: true}, sql.NullTime{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM invoices`).WithArgs(invID).WillReturnRows(invRows)

	payRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_z", nil, piID, "https://z", int64(200), "USD", "k3", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments`).WithArgs(orgID, invID).WillReturnRows(payRows)

	mock.ExpectExec(`UPDATE payments`).WithArgs("card_declined", payID).WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := RecordStripePaymentFailure(ctx, tx, StripePaymentFailureInput{
		StripePaymentIntentID: piID,
		FailureCode:           "card_declined",
	}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
