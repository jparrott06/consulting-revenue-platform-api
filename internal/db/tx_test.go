package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestRunInTx_CommitsOnSuccess(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	mock.ExpectBegin()
	mock.ExpectCommit()

	err = RunInTx(context.Background(), database, nil, func(*sql.Tx) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunInTx_RollsBackOnError(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	mock.ExpectBegin()
	mock.ExpectRollback()

	err = RunInTx(context.Background(), database, nil, func(*sql.Tx) error {
		return errors.New("boom")
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunInTx_RollsBackOnPanic(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	mock.ExpectBegin()
	mock.ExpectRollback()

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = RunInTx(context.Background(), database, nil, func(*sql.Tx) error {
		panic("panic")
	})
}

func TestRunInTx_BeginError(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()

	mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

	err = RunInTx(context.Background(), database, nil, func(*sql.Tx) error {
		return nil
	})
	if err == nil || err.Error() != "begin failed" {
		t.Fatalf("expected begin failed, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
