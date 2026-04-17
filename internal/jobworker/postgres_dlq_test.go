package jobworker

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresDLQ_Write(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`INSERT INTO jobs_dead_letter`).
		WithArgs("webhooks", []byte(`{"id":1}`), "failed", 3).
		WillReturnResult(sqlmock.NewResult(1, 1))

	dlq := &PostgresDLQ{DB: db}
	if err := dlq.Write(context.Background(), "webhooks", []byte(`{"id":1}`), 3, "failed"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
