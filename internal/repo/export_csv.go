package repo

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// InvoiceCSVHeader is the stable column order for invoices export.
var InvoiceCSVHeader = []string{
	"id",
	"invoice_number",
	"status",
	"currency",
	"subtotal_minor",
	"tax_minor",
	"total_minor",
	"issued_at_rfc3339",
	"due_at_rfc3339",
	"created_at_rfc3339",
	"updated_at_rfc3339",
}

// PaymentCSVHeader is the stable column order for payments export.
var PaymentCSVHeader = []string{
	"payment_id",
	"invoice_id",
	"invoice_number",
	"stripe_payment_link_id",
	"stripe_checkout_session_id",
	"stripe_payment_intent_id",
	"payment_url",
	"amount_minor",
	"currency",
	"idempotency_key",
	"refunded_amount_minor",
	"last_stripe_failure_code",
	"created_at_rfc3339",
	"updated_at_rfc3339",
}

// TimeSummaryCSVHeader is per-project rolled-up time for the organization.
var TimeSummaryCSVHeader = []string{
	"project_id",
	"project_name",
	"total_minutes",
	"time_entry_count",
}

// StreamInvoicesForCSV calls emit once for each data row (not including header); caller writes header.
func StreamInvoicesForCSV(ctx context.Context, db *sql.DB, organizationID uuid.UUID, emit func([]string) error) error {
	rows, err := db.QueryContext(ctx, `
SELECT id, invoice_number, status, currency, subtotal_minor, tax_minor, total_minor,
  issued_at, due_at, created_at, updated_at
FROM invoices
WHERE organization_id = $1
ORDER BY created_at ASC, id ASC`, organizationID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			id                                  uuid.UUID
			num                                 int64
			status, currency                    string
			subtotalMinor, taxMinor, totalMinor int64
			issuedAt, dueAt                     sql.NullTime
			createdAt, updatedAt                time.Time
		)
		if err := rows.Scan(
			&id, &num, &status, &currency,
			&subtotalMinor, &taxMinor, &totalMinor,
			&issuedAt, &dueAt,
			&createdAt, &updatedAt,
		); err != nil {
			return err
		}
		row := []string{
			id.String(),
			strconv.FormatInt(num, 10),
			status,
			currency,
			strconv.FormatInt(subtotalMinor, 10),
			strconv.FormatInt(taxMinor, 10),
			strconv.FormatInt(totalMinor, 10),
			formatNullRFC3339(issuedAt),
			formatNullRFC3339(dueAt),
			createdAt.UTC().Format(time.RFC3339Nano),
			updatedAt.UTC().Format(time.RFC3339Nano),
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	return rows.Err()
}

// StreamPaymentsForCSV emits payment rows joined with invoice numbers.
func StreamPaymentsForCSV(ctx context.Context, db *sql.DB, organizationID uuid.UUID, emit func([]string) error) error {
	rows, err := db.QueryContext(ctx, `
SELECT
  p.id, p.invoice_id, i.invoice_number, p.stripe_payment_link_id,
  p.stripe_checkout_session_id, p.stripe_payment_intent_id, p.payment_url,
  p.amount_minor, p.currency, p.idempotency_key, p.refunded_amount_minor, p.last_stripe_failure_code,
  p.created_at, p.updated_at
FROM payments p
INNER JOIN invoices i ON i.id = p.invoice_id AND i.organization_id = p.organization_id
WHERE p.organization_id = $1
ORDER BY p.created_at ASC, p.id ASC`, organizationID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			payID, invID         uuid.UUID
			invNum               int64
			linkID, payURL       string
			sessID, piID         sql.NullString
			amountMinor          int64
			currency, idem       string
			refunded             int64
			lastFail             sql.NullString
			createdAt, updatedAt time.Time
		)
		if err := rows.Scan(
			&payID, &invID, &invNum, &linkID,
			&sessID, &piID, &payURL,
			&amountMinor, &currency, &idem, &refunded, &lastFail,
			&createdAt, &updatedAt,
		); err != nil {
			return err
		}
		row := []string{
			payID.String(),
			invID.String(),
			strconv.FormatInt(invNum, 10),
			linkID,
			nullStringVal(sessID),
			nullStringVal(piID),
			payURL,
			strconv.FormatInt(amountMinor, 10),
			currency,
			idem,
			strconv.FormatInt(refunded, 10),
			nullStringVal(lastFail),
			createdAt.UTC().Format(time.RFC3339Nano),
			updatedAt.UTC().Format(time.RFC3339Nano),
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	return rows.Err()
}

// StreamTimeSummaryForCSV emits one row per project with summed minutes and entry counts.
func StreamTimeSummaryForCSV(ctx context.Context, db *sql.DB, organizationID uuid.UUID, emit func([]string) error) error {
	rows, err := db.QueryContext(ctx, `
SELECT
  pr.id,
  pr.name,
  COALESCE(SUM(te.minutes), 0)::bigint,
  COUNT(te.id)::bigint
FROM projects pr
LEFT JOIN time_entries te ON te.project_id = pr.id AND te.organization_id = pr.organization_id
WHERE pr.organization_id = $1
GROUP BY pr.id, pr.name
ORDER BY pr.name ASC, pr.id ASC`, organizationID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			pid     uuid.UUID
			name    string
			minutes int64
			entryCt int64
		)
		if err := rows.Scan(&pid, &name, &minutes, &entryCt); err != nil {
			return err
		}
		row := []string{
			pid.String(),
			name,
			strconv.FormatInt(minutes, 10),
			strconv.FormatInt(entryCt, 10),
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	return rows.Err()
}

func formatNullRFC3339(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339Nano)
}

func nullStringVal(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}
