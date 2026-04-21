package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// ReconciliationCurrencyRow compares ledger payment_captured totals to sums of payment rows
// for invoices in paid status, per currency. DriftMinor is ledger minus paid-invoice payments;
// zero means aligned for that currency.
type ReconciliationCurrencyRow struct {
	Currency                   string
	LedgerPaymentCapturedMinor int64
	PaidInvoicePaymentsMinor   int64
	DriftMinor                 int64
}

// ReconciliationSummary is a tenant-scoped read-only view for operators (cron or GET endpoint).
type ReconciliationSummary struct {
	Currencies []ReconciliationCurrencyRow
}

// GetReconciliationSummary returns per-currency Stripe payment capture alignment for one organization.
func GetReconciliationSummary(ctx context.Context, db *sql.DB, organizationID uuid.UUID) (*ReconciliationSummary, error) {
	rows, err := db.QueryContext(ctx, `
WITH ledger AS (
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
ORDER BY currency`, organizationID, LedgerEventPaymentCaptured)
	if err != nil {
		return nil, fmt.Errorf("reconciliation summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ReconciliationCurrencyRow
	for rows.Next() {
		var r ReconciliationCurrencyRow
		if err := rows.Scan(&r.Currency, &r.LedgerPaymentCapturedMinor, &r.PaidInvoicePaymentsMinor, &r.DriftMinor); err != nil {
			return nil, fmt.Errorf("reconciliation summary scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reconciliation summary rows: %w", err)
	}
	if out == nil {
		out = []ReconciliationCurrencyRow{}
	}
	return &ReconciliationSummary{Currencies: out}, nil
}
