package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrTimeEntryNotFound is returned when a time entry is missing in the organization scope.
var ErrTimeEntryNotFound = errors.New("time entry not found")

// ErrTimeEntryLocked is returned when the entry cannot be edited (approved/invoiced).
var ErrTimeEntryLocked = errors.New("time entry is locked for editing")

// TimeEntryRecord is a tenant-scoped time entry row.
type TimeEntryRecord struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	ProjectID       uuid.UUID
	UserID          uuid.UUID
	WorkDate        time.Time
	Minutes         int
	HourlyRateMinor int64
	Status          string
	Notes           sql.NullString
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ListTimeEntryFilters narrows time entry listing.
type ListTimeEntryFilters struct {
	ProjectID *uuid.UUID
	UserID    *uuid.UUID
	Status    *string
	From      *time.Time
	To        *time.Time
}

// ValidateProjectForTimeEntry ensures the project belongs to the org and is not archived.
func ValidateProjectForTimeEntry(ctx context.Context, db *sql.DB, organizationID, projectID uuid.UUID) error {
	p, err := GetProject(ctx, db, organizationID, projectID)
	if err != nil {
		return err
	}
	if p.Archived {
		return errors.New("project is archived")
	}
	return nil
}

// CreateTimeEntry inserts a draft time entry.
func CreateTimeEntry(ctx context.Context, db *sql.DB, organizationID, projectID, userID uuid.UUID, workDate time.Time, minutes int, hourlyRateMinor int64, notes *string) (uuid.UUID, error) {
	if err := ValidateProjectForTimeEntry(ctx, db, organizationID, projectID); err != nil {
		return uuid.Nil, err
	}
	if minutes <= 0 || minutes > 1440 {
		return uuid.Nil, errors.New("minutes must be between 1 and 1440 inclusive")
	}
	if hourlyRateMinor < 0 {
		return uuid.Nil, errors.New("hourly_rate_minor must be non-negative")
	}

	var notesVal sql.NullString
	if notes != nil {
		s := strings.TrimSpace(*notes)
		if s != "" {
			notesVal = sql.NullString{String: s, Valid: true}
		}
	}

	var id uuid.UUID
	err := db.QueryRowContext(ctx, `
INSERT INTO time_entries (organization_id, project_id, user_id, work_date, minutes, hourly_rate_minor, status, notes)
VALUES ($1, $2, $3, $4, $5, $6, 'draft', $7)
RETURNING id`,
		organizationID, projectID, userID, workDate, minutes, hourlyRateMinor, notesVal,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// GetTimeEntry loads a time entry scoped to the organization.
func GetTimeEntry(ctx context.Context, db *sql.DB, organizationID, entryID uuid.UUID) (TimeEntryRecord, error) {
	var rec TimeEntryRecord
	err := db.QueryRowContext(ctx, `
SELECT id, organization_id, project_id, user_id, work_date, minutes, hourly_rate_minor, status, notes, created_at, updated_at
FROM time_entries
WHERE id = $1 AND organization_id = $2`, entryID, organizationID,
	).Scan(
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
	)
	if errors.Is(err, sql.ErrNoRows) {
		return TimeEntryRecord{}, ErrTimeEntryNotFound
	}
	return rec, err
}

// ListTimeEntries returns time entries for an organization with optional filters.
func ListTimeEntries(ctx context.Context, db *sql.DB, organizationID uuid.UUID, filters ListTimeEntryFilters) ([]TimeEntryRecord, error) {
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
	}
	q += ` ORDER BY work_date ASC, id ASC`

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimeEntryRecord
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
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// UpdateTimeEntry patches fields when status is draft or submitted (not approved/invoiced).
func UpdateTimeEntry(ctx context.Context, db *sql.DB, organizationID, entryID uuid.UUID, projectID *uuid.UUID, workDate *time.Time, minutes *int, hourlyRateMinor *int64, notes *string) error {
	rec, err := GetTimeEntry(ctx, db, organizationID, entryID)
	if err != nil {
		return err
	}
	if rec.Status == "approved" || rec.Status == "invoiced" {
		return ErrTimeEntryLocked
	}
	if rec.Status != "draft" && rec.Status != "submitted" {
		return ErrTimeEntryLocked
	}

	newProject := rec.ProjectID
	if projectID != nil {
		newProject = *projectID
		if err := ValidateProjectForTimeEntry(ctx, db, organizationID, newProject); err != nil {
			return err
		}
	}
	newDate := rec.WorkDate
	if workDate != nil {
		newDate = *workDate
	}
	newMinutes := rec.Minutes
	if minutes != nil {
		if *minutes <= 0 || *minutes > 1440 {
			return errors.New("minutes must be between 1 and 1440 inclusive")
		}
		newMinutes = *minutes
	}
	newRate := rec.HourlyRateMinor
	if hourlyRateMinor != nil {
		if *hourlyRateMinor < 0 {
			return errors.New("hourly_rate_minor must be non-negative")
		}
		newRate = *hourlyRateMinor
	}
	var notesVal sql.NullString
	if notes != nil {
		s := strings.TrimSpace(*notes)
		if s == "" {
			notesVal = sql.NullString{Valid: false}
		} else {
			notesVal = sql.NullString{String: s, Valid: true}
		}
	} else {
		notesVal = rec.Notes
	}

	res, err := db.ExecContext(ctx, `
UPDATE time_entries
SET project_id = $1, work_date = $2, minutes = $3, hourly_rate_minor = $4, notes = $5, updated_at = NOW()
WHERE id = $6 AND organization_id = $7 AND status IN ('draft', 'submitted')`,
		newProject, newDate, newMinutes, newRate, notesVal, entryID, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTimeEntryNotFound
	}
	return nil
}
