package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// RunInTx executes fn inside a single database transaction.
// If fn returns a non-nil error, the transaction is rolled back and that error is returned.
// If fn returns nil, the transaction is committed; a commit error is returned.
// Panics are recovered: the transaction is rolled back and the panic is re-raised.
func RunInTx(ctx context.Context, database *sql.DB, opts *sql.TxOptions, fn func(*sql.Tx) error) (err error) {
	tx, err := database.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return fmt.Errorf("%w (rollback: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
