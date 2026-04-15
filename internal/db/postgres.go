package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

const pingTimeout = 5 * time.Second

// OpenPostgres creates and validates a PostgreSQL connection pool.
func OpenPostgres(ctx context.Context, databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required to connect to postgres")
	}

	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return conn, nil
}
