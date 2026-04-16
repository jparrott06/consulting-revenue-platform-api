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

func TestSendInvoice_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	invID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE invoices`).
		WithArgs(invID, orgID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT id, organization_id, invoice_number, status, currency, subtotal_minor, tax_minor, total_minor, issued_at, due_at, created_at, updated_at
FROM invoices
WHERE id = \$1 AND organization_id = \$2`).
		WithArgs(invID, orgID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = SendInvoice(context.Background(), db, orgID, invID)
	if !errors.Is(err, ErrInvoiceNotFound) {
		t.Fatalf("expected ErrInvoiceNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSendInvoice_IdempotentIssued(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	invID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	issued := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE invoices`).
		WithArgs(invID, orgID).
		WillReturnError(sql.ErrNoRows)
	rows := sqlmock.NewRows([]string{
		"id", "organization_id", "invoice_number", "status", "currency",
		"subtotal_minor", "tax_minor", "total_minor", "issued_at", "due_at", "created_at", "updated_at",
	}).AddRow(
		invID, orgID, int64(1), "issued", "USD",
		int64(100), int64(0), int64(100), issued, nil, issued, issued,
	)
	mock.ExpectQuery(`SELECT id, organization_id, invoice_number, status, currency, subtotal_minor, tax_minor, total_minor, issued_at, due_at, created_at, updated_at
FROM invoices
WHERE id = \$1 AND organization_id = \$2`).
		WithArgs(invID, orgID).
		WillReturnRows(rows)
	mock.ExpectCommit()

	rec, err := SendInvoice(context.Background(), db, orgID, invID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "issued" {
		t.Fatalf("status %q", rec.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
