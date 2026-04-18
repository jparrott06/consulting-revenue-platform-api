package repo

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestReconcileStripePaymentPaid_AlreadyPaid(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	invID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_number", "status", "currency", "subtotal_minor", "tax_minor", "total_minor",
		"issued_at", "due_at", "created_at", "updated_at",
	}).AddRow(invID, orgID, int64(1), "paid", "USD", int64(100), int64(0), int64(100),
		sql.NullTime{Time: time.Now(), Valid: true}, sql.NullTime{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM invoices`).WithArgs(invID).WillReturnRows(rows)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	err = ReconcileStripePaymentPaid(ctx, tx, StripePaidReconcileInput{InvoiceID: invID})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileStripePaymentPaid_IssuedHappyPath(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	invID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	orgID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	payID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	linkID := "plink_test123"

	mock.ExpectBegin()

	invRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_number", "status", "currency", "subtotal_minor", "tax_minor", "total_minor",
		"issued_at", "due_at", "created_at", "updated_at",
	}).AddRow(invID, orgID, int64(2), "issued", "USD", int64(5000), int64(0), int64(5000),
		sql.NullTime{Time: time.Now(), Valid: true}, sql.NullTime{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM invoices`).WithArgs(invID).WillReturnRows(invRows)

	payRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, linkID, nil, nil, "https://pay.example", int64(5000), "USD", "key", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments`).WithArgs(orgID, invID).WillReturnRows(payRows)

	mock.ExpectExec(`UPDATE invoices`).WithArgs(invID, orgID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE payments`).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), payID).WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	in := StripePaidReconcileInput{
		InvoiceID:               invID,
		StripePaymentLinkID:     linkID,
		StripeCheckoutSessionID: "cs_test_1",
		StripePaymentIntentID:   "pi_test_1",
		AmountMinor:             5000,
		Currency:                "usd",
	}
	if err := ReconcileStripePaymentPaid(ctx, tx, in); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileStripePaymentPaid_CurrencyMismatch(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	invID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	orgID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	payID := uuid.MustParse("88888888-8888-8888-8888-888888888888")

	mock.ExpectBegin()

	invRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_number", "status", "currency", "subtotal_minor", "tax_minor", "total_minor",
		"issued_at", "due_at", "created_at", "updated_at",
	}).AddRow(invID, orgID, int64(3), "issued", "USD", int64(100), int64(0), int64(100),
		sql.NullTime{Valid: true}, sql.NullTime{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM invoices`).WithArgs(invID).WillReturnRows(invRows)

	payRows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_id", "stripe_payment_link_id", "stripe_checkout_session_id", "stripe_payment_intent_id",
		"payment_url", "amount_minor", "currency", "idempotency_key", "refunded_amount_minor", "last_stripe_failure_code", "created_at", "updated_at",
	}).AddRow(payID, orgID, invID, "plink_x", nil, nil, "https://pay.example", int64(100), "USD", "key", int64(0), sql.NullString{}, time.Now(), time.Now())
	mock.ExpectQuery(`FROM payments`).WithArgs(orgID, invID).WillReturnRows(payRows)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	err = ReconcileStripePaymentPaid(ctx, tx, StripePaidReconcileInput{
		InvoiceID:           invID,
		StripePaymentLinkID: "plink_x",
		AmountMinor:         100,
		Currency:            "jpy",
	})
	if !errors.Is(err, ErrStripeReconcileCurrencyMismatch) {
		t.Fatalf("expected currency mismatch error, got %v", err)
	}
}

func TestResolveStripePaidInvoiceID_LinkOnly(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	invID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT invoice_id FROM payments`).
		WithArgs("plink_only").
		WillReturnRows(sqlmock.NewRows([]string{"invoice_id"}).AddRow(invID))

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	got, err := ResolveStripePaidInvoiceID(ctx, tx, "", "plink_only")
	if err != nil {
		t.Fatal(err)
	}
	if got != invID {
		t.Fatalf("got %v want %v", got, invID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
