package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrReportYearOutOfRange indicates an invalid year selector for reporting.
var ErrReportYearOutOfRange = errors.New("year out of range")

// ErrReportMonthOutOfRange indicates an invalid month selector for reporting.
var ErrReportMonthOutOfRange = errors.New("month out of range")

// CurrencyTotal is one currency bucket for a report.
type CurrencyTotal struct {
	AmountMinor  int64 `json:"amount_minor"`
	InvoiceCount int64 `json:"invoice_count"`
}

// OutstandingReport is issued, unpaid invoice totals by currency.
type OutstandingReport struct {
	AsOf       time.Time                `json:"as_of"`
	Timezone   string                   `json:"timezone"`
	ByCurrency map[string]CurrencyTotal `json:"by_currency"`
}

// PaidThisMonthReport sums invoices marked paid in a UTC calendar month.
type PaidThisMonthReport struct {
	Year       int                      `json:"year"`
	Month      int                      `json:"month"`
	Timezone   string                   `json:"timezone"`
	ByCurrency map[string]CurrencyTotal `json:"by_currency"`
}

// AgingBucketTotals breaks issued open AR by past-due buckets (amounts in minor units).
type AgingBucketTotals struct {
	CurrentMinor    int64 `json:"current_minor"`
	Days1To30Minor  int64 `json:"days_1_30_minor"`
	Days31To60Minor int64 `json:"days_31_60_minor"`
	Days61To90Minor int64 `json:"days_61_90_minor"`
	DaysOver90Minor int64 `json:"days_over_90_minor"`
}

// AgingReport is outstanding issued invoices grouped by aging buckets per currency.
type AgingReport struct {
	AsOf       time.Time                    `json:"as_of"`
	Timezone   string                       `json:"timezone"`
	ByCurrency map[string]AgingBucketTotals `json:"by_currency"`
}

// ReportOutstandingIssued returns totals for invoices in issued status (open AR).
func ReportOutstandingIssued(ctx context.Context, db *sql.DB, organizationID uuid.UUID) (OutstandingReport, error) {
	rows, err := db.QueryContext(ctx, `
SELECT currency, COALESCE(SUM(total_minor), 0)::bigint, COUNT(*)::bigint
FROM invoices
WHERE organization_id = $1 AND status = 'issued'
GROUP BY currency`, organizationID)
	if err != nil {
		return OutstandingReport{}, err
	}
	defer func() { _ = rows.Close() }()

	out := OutstandingReport{
		AsOf:       time.Now().UTC(),
		Timezone:   "UTC",
		ByCurrency: map[string]CurrencyTotal{},
	}
	for rows.Next() {
		var cur string
		var sum, n int64
		if err := rows.Scan(&cur, &sum, &n); err != nil {
			return OutstandingReport{}, err
		}
		out.ByCurrency[cur] = CurrencyTotal{AmountMinor: sum, InvoiceCount: n}
	}
	return out, rows.Err()
}

// ReportPaidInUTCMonth returns totals for invoices transitioned to paid in [start, end).
func ReportPaidInUTCMonth(ctx context.Context, db *sql.DB, organizationID uuid.UUID, year int, month time.Month) (PaidThisMonthReport, error) {
	if year < 2000 || year > 2100 {
		return PaidThisMonthReport{}, ErrReportYearOutOfRange
	}
	if month < 1 || month > 12 {
		return PaidThisMonthReport{}, ErrReportMonthOutOfRange
	}
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	rows, err := db.QueryContext(ctx, `
SELECT currency, COALESCE(SUM(total_minor), 0)::bigint, COUNT(*)::bigint
FROM invoices
WHERE organization_id = $1 AND status = 'paid'
  AND updated_at >= $2 AND updated_at < $3
GROUP BY currency`, organizationID, start, end)
	if err != nil {
		return PaidThisMonthReport{}, err
	}
	defer func() { _ = rows.Close() }()

	out := PaidThisMonthReport{
		Year:       year,
		Month:      int(month),
		Timezone:   "UTC",
		ByCurrency: map[string]CurrencyTotal{},
	}
	for rows.Next() {
		var cur string
		var sum, n int64
		if err := rows.Scan(&cur, &sum, &n); err != nil {
			return PaidThisMonthReport{}, err
		}
		out.ByCurrency[cur] = CurrencyTotal{AmountMinor: sum, InvoiceCount: n}
	}
	return out, rows.Err()
}

// ReportAgingIssued buckets open (issued) invoices by days past due reference date.
func ReportAgingIssued(ctx context.Context, db *sql.DB, organizationID uuid.UUID) (AgingReport, error) {
	rows, err := db.QueryContext(ctx, `
SELECT currency,
  COALESCE(SUM(CASE WHEN ref_date >= today THEN total_minor ELSE 0 END), 0)::bigint,
  COALESCE(SUM(CASE WHEN ref_date < today AND (today - ref_date) BETWEEN 1 AND 30 THEN total_minor ELSE 0 END), 0)::bigint,
  COALESCE(SUM(CASE WHEN ref_date < today AND (today - ref_date) BETWEEN 31 AND 60 THEN total_minor ELSE 0 END), 0)::bigint,
  COALESCE(SUM(CASE WHEN ref_date < today AND (today - ref_date) BETWEEN 61 AND 90 THEN total_minor ELSE 0 END), 0)::bigint,
  COALESCE(SUM(CASE WHEN ref_date < today AND (today - ref_date) > 90 THEN total_minor ELSE 0 END), 0)::bigint
FROM (
  SELECT currency, total_minor,
    (timezone('UTC', COALESCE(due_at, issued_at, created_at)))::date AS ref_date
  FROM invoices
  WHERE organization_id = $1 AND status = 'issued'
) inv
CROSS JOIN (
  SELECT (timezone('UTC', now()))::date AS today
) cut
GROUP BY currency`, organizationID)
	if err != nil {
		return AgingReport{}, err
	}
	defer func() { _ = rows.Close() }()

	out := AgingReport{
		AsOf:       time.Now().UTC(),
		Timezone:   "UTC",
		ByCurrency: map[string]AgingBucketTotals{},
	}
	for rows.Next() {
		var cur string
		var curM, b1, b2, b3, b4 int64
		if err := rows.Scan(&cur, &curM, &b1, &b2, &b3, &b4); err != nil {
			return AgingReport{}, err
		}
		out.ByCurrency[cur] = AgingBucketTotals{
			CurrentMinor:    curM,
			Days1To30Minor:  b1,
			Days31To60Minor: b2,
			Days61To90Minor: b3,
			DaysOver90Minor: b4,
		}
	}
	return out, rows.Err()
}
