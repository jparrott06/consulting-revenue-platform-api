package retention

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

func TestRunOnce_InvokesBothPurges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cfg := config.Config{
		RetentionAuditLogDays:     10,
		RetentionWebhookEventDays: 20,
	}

	mock.ExpectExec(`DELETE FROM audit_logs WHERE created_at < \$1`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM webhook_events WHERE received_at < \$1`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))

	a, w, err := RunOnce(context.Background(), db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if a != 1 || w != 2 {
		t.Fatalf("got audit=%d webhook=%d", a, w)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunOnce_DefaultDaysWhenZero(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cfg := config.Config{}

	mock.ExpectExec(`DELETE FROM audit_logs WHERE created_at < \$1`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`DELETE FROM webhook_events WHERE received_at < \$1`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, _, err = RunOnce(context.Background(), db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
