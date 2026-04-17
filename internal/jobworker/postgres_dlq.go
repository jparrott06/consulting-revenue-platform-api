package jobworker

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

// PostgresDLQ persists exhausted jobs using repo.InsertDeadLetter.
type PostgresDLQ struct {
	DB *sql.DB
}

// Write implements DLQWriter.
func (p *PostgresDLQ) Write(ctx context.Context, queue string, payload []byte, attempt int, errText string) error {
	if p == nil || p.DB == nil {
		return errors.New("postgres dlq: database is not configured")
	}
	return repo.InsertDeadLetter(ctx, p.DB, queue, payload, attempt, errText)
}
