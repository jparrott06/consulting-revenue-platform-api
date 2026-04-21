package repo

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestGetReconciliationSummary_Empty(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rows := sqlmock.NewRows([]string{"currency", "ledger_minor", "paid_minor", "drift_minor"})
	mock.ExpectQuery(`WITH ledger AS (
  SELECT currency, COALESCE(SUM(amount_minor), 0)::bigint AS sum_minor
  FROM ledger_entries
  WHERE organization_id = $1 AND event_type = $2
  GROUP BY currency
),
paid AS (
  SELECT p.currency, COALESCE(SUM(p.amount_minor), 0)::bigint AS sum_minor
  FROM payments p
  INNER JOIN invoices i ON i.id = p.invoice_id AND i.organization_id = p.organization_id
  WHERE p.organization_id = $1 AND i.status = 'paid'
  GROUP BY p.currency
)
SELECT
  COALESCE(l.currency, p.currency) AS currency,
  COALESCE(l.sum_minor, 0)::bigint AS ledger_minor,
  COALESCE(p.sum_minor, 0)::bigint AS paid_minor,
  (COALESCE(l.sum_minor, 0) - COALESCE(p.sum_minor, 0))::bigint AS drift_minor
FROM ledger l
FULL OUTER JOIN paid p ON l.currency = p.currency
ORDER BY currency`).WithArgs(orgID, LedgerEventPaymentCaptured).WillReturnRows(rows)

	got, err := GetReconciliationSummary(context.Background(), db, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Currencies) != 0 {
		t.Fatalf("expected no rows, got %d", len(got.Currencies))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetReconciliationSummary_DriftUSD(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rows := sqlmock.NewRows([]string{"currency", "ledger_minor", "paid_minor", "drift_minor"}).
		AddRow("USD", int64(5000), int64(10000), int64(-5000))
	mock.ExpectQuery(`WITH ledger AS (
  SELECT currency, COALESCE(SUM(amount_minor), 0)::bigint AS sum_minor
  FROM ledger_entries
  WHERE organization_id = $1 AND event_type = $2
  GROUP BY currency
),
paid AS (
  SELECT p.currency, COALESCE(SUM(p.amount_minor), 0)::bigint AS sum_minor
  FROM payments p
  INNER JOIN invoices i ON i.id = p.invoice_id AND i.organization_id = p.organization_id
  WHERE p.organization_id = $1 AND i.status = 'paid'
  GROUP BY p.currency
)
SELECT
  COALESCE(l.currency, p.currency) AS currency,
  COALESCE(l.sum_minor, 0)::bigint AS ledger_minor,
  COALESCE(p.sum_minor, 0)::bigint AS paid_minor,
  (COALESCE(l.sum_minor, 0) - COALESCE(p.sum_minor, 0))::bigint AS drift_minor
FROM ledger l
FULL OUTER JOIN paid p ON l.currency = p.currency
ORDER BY currency`).WithArgs(orgID, LedgerEventPaymentCaptured).WillReturnRows(rows)

	got, err := GetReconciliationSummary(context.Background(), db, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Currencies) != 1 {
		t.Fatalf("got %d rows", len(got.Currencies))
	}
	r := got.Currencies[0]
	if r.Currency != "USD" || r.LedgerPaymentCapturedMinor != 5000 || r.PaidInvoicePaymentsMinor != 10000 || r.DriftMinor != -5000 {
		t.Fatalf("unexpected row: %+v", r)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
