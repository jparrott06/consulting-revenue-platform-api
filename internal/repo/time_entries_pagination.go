package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TimeEntryCursor struct {
	WorkDate time.Time
	ID       uuid.UUID
}

func ListTimeEntriesPage(ctx context.Context, db *sql.DB, organizationID uuid.UUID, filters ListTimeEntryFilters, cursor *TimeEntryCursor, limit int) ([]TimeEntryRecord, *TimeEntryCursor, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	q := `
SELECT id, organization_id, project_id, user_id, work_date, minutes, hourly_rate_minor, status, notes, created_at, updated_at
FROM time_entries
WHERE organization_id = $1`
	args := []interface{}{organizationID}
	n := 2
	if filters.ProjectID != nil {
		q += fmt.Sprintf(" AND project_id = $%d", n)
		args = append(args, *filters.ProjectID)
		n++
	}
	if filters.UserID != nil {
		q += fmt.Sprintf(" AND user_id = $%d", n)
		args = append(args, *filters.UserID)
		n++
	}
	if filters.Status != nil {
		q += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, *filters.Status)
		n++
	}
	if filters.From != nil {
		q += fmt.Sprintf(" AND work_date >= $%d", n)
		args = append(args, filters.From.Format("2006-01-02"))
		n++
	}
	if filters.To != nil {
		q += fmt.Sprintf(" AND work_date <= $%d", n)
		args = append(args, filters.To.Format("2006-01-02"))
		n++
	}
	if cursor != nil {
		q += fmt.Sprintf(" AND (work_date > $%d OR (work_date = $%d AND id > $%d))", n, n, n+1)
		args = append(args, cursor.WorkDate.Format("2006-01-02"), cursor.ID)
		n += 2
	}
	q += fmt.Sprintf(" ORDER BY work_date ASC, id ASC LIMIT %d", limit+1)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]TimeEntryRecord, 0, limit+1)
	for rows.Next() {
		var rec TimeEntryRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.OrganizationID,
			&rec.ProjectID,
			&rec.UserID,
			&rec.WorkDate,
			&rec.Minutes,
			&rec.HourlyRateMinor,
			&rec.Status,
			&rec.Notes,
			&rec.CreatedAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	if len(out) <= limit {
		return out, nil, nil
	}
	nextRow := out[limit-1]
	next := &TimeEntryCursor{WorkDate: nextRow.WorkDate, ID: nextRow.ID}
	return out[:limit], next, nil
}
