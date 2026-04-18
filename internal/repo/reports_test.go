package repo

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestReportOutstandingIssued_Sums(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orgID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	rows := sqlmock.NewRows([]string{"currency", "sum", "count"}).
		AddRow("USD", int64(5000), int64(2))
	mock.ExpectQuery(`FROM invoices`).WithArgs(orgID).WillReturnRows(rows)

	got, err := ReportOutstandingIssued(context.Background(), db, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ByCurrency["USD"].AmountMinor != 5000 || got.ByCurrency["USD"].InvoiceCount != 2 {
		t.Fatalf("unexpected outstanding: %#v", got.ByCurrency["USD"])
	}
	if got.Timezone != "UTC" {
		t.Fatalf("timezone: %q", got.Timezone)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReportPaidInUTCMonth_Empty(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orgID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	rows := sqlmock.NewRows([]string{"currency", "sum", "count"})
	mock.ExpectQuery(`FROM invoices`).WithArgs(orgID, start, end).WillReturnRows(rows)

	got, err := ReportPaidInUTCMonth(context.Background(), db, orgID, 2026, time.April)
	if err != nil {
		t.Fatal(err)
	}
	if got.Year != 2026 || got.Month != 4 {
		t.Fatalf("year/month: %d-%d", got.Year, got.Month)
	}
	if len(got.ByCurrency) != 0 {
		t.Fatalf("expected empty: %#v", got.ByCurrency)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReportPaidInUTCMonth_InvalidMonth(t *testing.T) {
	t.Parallel()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orgID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	_, err = ReportPaidInUTCMonth(context.Background(), db, orgID, 2020, time.Month(0))
	if err != ErrReportMonthOutOfRange {
		t.Fatalf("got %v", err)
	}
}

func TestReportAgingIssued_Buckets(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orgID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	rows := sqlmock.NewRows([]string{"currency", "cur", "b1", "b2", "b3", "b4"}).
		AddRow("USD", int64(100), int64(50), int64(25), int64(0), int64(10))

	mock.ExpectQuery(`FROM invoices`).WithArgs(orgID).WillReturnRows(rows)

	got, err := ReportAgingIssued(context.Background(), db, orgID)
	if err != nil {
		t.Fatal(err)
	}
	a := got.ByCurrency["USD"]
	if a.CurrentMinor != 100 || a.Days1To30Minor != 50 || a.Days31To60Minor != 25 || a.Days61To90Minor != 0 || a.DaysOver90Minor != 10 {
		t.Fatalf("unexpected aging: %#v", a)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
