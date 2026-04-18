package webhookworker

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func TestProcessOne_NoPendingEvent_Rollbacks(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, event_id`).
		WithArgs(5).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	cfg := config.Config{WebhookWorkerMaxAttempts: 5}
	err = ProcessOne(context.Background(), db, cfg)
	if !errors.Is(err, repo.ErrNoPendingWebhookEvent) {
		t.Fatalf("expected ErrNoPendingWebhookEvent, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
