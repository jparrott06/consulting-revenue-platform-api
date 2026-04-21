package integrationtest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/db"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration tests")
	}
	ctx := context.Background()
	pool, err := db.OpenPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%s@test.local", prefix, uuid.NewString())
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func execSQL(t *testing.T, pool *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := pool.ExecContext(context.Background(), query, args...)
	if err != nil {
		t.Fatal(err)
	}
}

// TestRollback_TimeEntrySubmit_EventInsertFails ensures a failure inserting the audit row rolls back status change.
func TestRollback_TimeEntrySubmit_EventInsertFails(t *testing.T) {
	ctx := context.Background()
	pool := openIntegrationDB(t)

	_, orgStr, err := repo.RegisterUserAndOrganization(ctx, pool, uniqueEmail("owner"), mustHash(t, "p"), "Owner")
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse(orgStr)

	var contractorID uuid.UUID
	err = pool.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
		uniqueEmail("contractor"), mustHash(t, "p"), "Contractor",
	).Scan(&contractorID)
	if err != nil {
		t.Fatal(err)
	}
	execSQL(t, pool, `
INSERT INTO memberships (user_id, organization_id, role, status, joined_at)
VALUES ($1, $2, 'contractor', 'active', NOW())`, contractorID, orgID)

	clientID, err := repo.CreateClient(ctx, pool, orgID, "Client", "c@test.local", "USD")
	if err != nil {
		t.Fatal(err)
	}
	projectID, err := repo.CreateProject(ctx, pool, orgID, clientID, "Proj", "hourly", 10000)
	if err != nil {
		t.Fatal(err)
	}
	workDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	entryID, err := repo.CreateTimeEntry(ctx, pool, orgID, projectID, contractorID, workDate, 60, 10000, nil)
	if err != nil {
		t.Fatal(err)
	}

	q := fmt.Sprintf(`
CREATE OR REPLACE FUNCTION int_test_fail_time_entry_events()
RETURNS trigger AS $$
BEGIN
  IF NEW.organization_id = '%s'::uuid THEN
    RAISE EXCEPTION 'injected_time_entry_event';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
`, orgID.String())
	execSQL(t, pool, q)
	execSQL(t, pool, `DROP TRIGGER IF EXISTS int_test_time_entry_events ON time_entry_events`)
	execSQL(t, pool, `
CREATE TRIGGER int_test_time_entry_events
BEFORE INSERT ON time_entry_events
FOR EACH ROW EXECUTE FUNCTION int_test_fail_time_entry_events()`)
	t.Cleanup(func() {
		_, _ = pool.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS int_test_time_entry_events ON time_entry_events`)
		_, _ = pool.ExecContext(context.Background(), `DROP FUNCTION IF EXISTS int_test_fail_time_entry_events()`)
	})

	err = repo.SubmitTimeEntry(ctx, pool, orgID, entryID, contractorID)
	if err == nil || !strings.Contains(err.Error(), "injected_time_entry_event") {
		t.Fatalf("expected injected failure, got %v", err)
	}

	var status string
	err = pool.QueryRowContext(ctx, `SELECT status FROM time_entries WHERE id = $1 AND organization_id = $2`, entryID, orgID).Scan(&status)
	if err != nil {
		t.Fatal(err)
	}
	if status != "draft" {
		t.Fatalf("expected draft after rollback, got %q", status)
	}
}

// TestRollback_InvoiceGenerate_LineItemInsertFails ensures no partial invoice when line items fail mid-transaction.
func TestRollback_InvoiceGenerate_LineItemInsertFails(t *testing.T) {
	ctx := context.Background()
	pool := openIntegrationDB(t)

	ownerStr, orgStr, err := repo.RegisterUserAndOrganization(ctx, pool, uniqueEmail("own2"), mustHash(t, "p"), "Owner")
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse(orgStr)
	ownerID := uuid.MustParse(ownerStr)

	var contractorID uuid.UUID
	err = pool.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
		uniqueEmail("ctr2"), mustHash(t, "p"), "Contractor",
	).Scan(&contractorID)
	if err != nil {
		t.Fatal(err)
	}
	execSQL(t, pool, `
INSERT INTO memberships (user_id, organization_id, role, status, joined_at)
VALUES ($1, $2, 'contractor', 'active', NOW())`, contractorID, orgID)

	clientID, err := repo.CreateClient(ctx, pool, orgID, "C2", "c2@test.local", "USD")
	if err != nil {
		t.Fatal(err)
	}
	projectID, err := repo.CreateProject(ctx, pool, orgID, clientID, "P2", "hourly", 10000)
	if err != nil {
		t.Fatal(err)
	}
	d1 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	for _, wd := range []time.Time{d1, d2} {
		eid, err := repo.CreateTimeEntry(ctx, pool, orgID, projectID, contractorID, wd, 120, 10000, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SubmitTimeEntry(ctx, pool, orgID, eid, contractorID); err != nil {
			t.Fatal(err)
		}
		if err := repo.ApproveTimeEntry(ctx, pool, orgID, eid, ownerID); err != nil {
			t.Fatal(err)
		}
	}

	var invCount int
	if err := pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE organization_id = $1`, orgID).Scan(&invCount); err != nil {
		t.Fatal(err)
	}

	q := fmt.Sprintf(`
CREATE OR REPLACE FUNCTION int_test_fail_invoice_lines()
RETURNS trigger AS $$
BEGIN
  IF NEW.organization_id = '%s'::uuid THEN
    RAISE EXCEPTION 'injected_invoice_line';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
`, orgID.String())
	execSQL(t, pool, q)
	execSQL(t, pool, `DROP TRIGGER IF EXISTS int_test_invoice_lines ON invoice_line_items`)
	execSQL(t, pool, `
CREATE TRIGGER int_test_invoice_lines
BEFORE INSERT ON invoice_line_items
FOR EACH ROW EXECUTE FUNCTION int_test_fail_invoice_lines()`)
	t.Cleanup(func() {
		_, _ = pool.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS int_test_invoice_lines ON invoice_line_items`)
		_, _ = pool.ExecContext(context.Background(), `DROP FUNCTION IF EXISTS int_test_fail_invoice_lines()`)
	})

	_, err = repo.GenerateInvoiceFromApprovedEntries(ctx, pool, orgID, repo.GenerateInvoiceParams{
		FromDate: d1,
		ToDate:   d2,
		Currency: "USD",
	})
	if err == nil || !strings.Contains(err.Error(), "injected_invoice_line") {
		t.Fatalf("expected injected failure, got %v", err)
	}

	var invCountAfter int
	if err := pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE organization_id = $1`, orgID).Scan(&invCountAfter); err != nil {
		t.Fatal(err)
	}
	if invCountAfter != invCount {
		t.Fatalf("invoice count changed after rollback: before=%d after=%d", invCount, invCountAfter)
	}
}

// TestRollback_InvoiceSend_LedgerInsertFails keeps invoice draft when ledger append fails.
func TestRollback_InvoiceSend_LedgerInsertFails(t *testing.T) {
	ctx := context.Background()
	pool := openIntegrationDB(t)

	ownerStr, orgStr, err := repo.RegisterUserAndOrganization(ctx, pool, uniqueEmail("own3"), mustHash(t, "p"), "Owner")
	if err != nil {
		t.Fatal(err)
	}
	orgID := uuid.MustParse(orgStr)
	ownerID := uuid.MustParse(ownerStr)

	var contractorID uuid.UUID
	err = pool.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
		uniqueEmail("ctr3"), mustHash(t, "p"), "Contractor",
	).Scan(&contractorID)
	if err != nil {
		t.Fatal(err)
	}
	execSQL(t, pool, `
INSERT INTO memberships (user_id, organization_id, role, status, joined_at)
VALUES ($1, $2, 'contractor', 'active', NOW())`, contractorID, orgID)

	clientID, err := repo.CreateClient(ctx, pool, orgID, "C3", "c3@test.local", "USD")
	if err != nil {
		t.Fatal(err)
	}
	projectID, err := repo.CreateProject(ctx, pool, orgID, clientID, "P3", "hourly", 10000)
	if err != nil {
		t.Fatal(err)
	}
	wd := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	entryID, err := repo.CreateTimeEntry(ctx, pool, orgID, projectID, contractorID, wd, 60, 10000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SubmitTimeEntry(ctx, pool, orgID, entryID, contractorID); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApproveTimeEntry(ctx, pool, orgID, entryID, ownerID); err != nil {
		t.Fatal(err)
	}

	inv, err := repo.GenerateInvoiceFromApprovedEntries(ctx, pool, orgID, repo.GenerateInvoiceParams{
		FromDate: wd,
		ToDate:   wd,
		Currency: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != "draft" {
		t.Fatalf("expected draft invoice, got %q", inv.Status)
	}

	q := fmt.Sprintf(`
CREATE OR REPLACE FUNCTION int_test_fail_ledger()
RETURNS trigger AS $$
BEGIN
  IF NEW.organization_id = '%s'::uuid THEN
    RAISE EXCEPTION 'injected_ledger';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
`, orgID.String())
	execSQL(t, pool, q)
	execSQL(t, pool, `DROP TRIGGER IF EXISTS int_test_ledger ON ledger_entries`)
	execSQL(t, pool, `
CREATE TRIGGER int_test_ledger
BEFORE INSERT ON ledger_entries
FOR EACH ROW EXECUTE FUNCTION int_test_fail_ledger()`)
	t.Cleanup(func() {
		_, _ = pool.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS int_test_ledger ON ledger_entries`)
		_, _ = pool.ExecContext(context.Background(), `DROP FUNCTION IF EXISTS int_test_fail_ledger()`)
	})

	_, err = repo.SendInvoice(ctx, pool, orgID, inv.ID)
	if err == nil || !strings.Contains(err.Error(), "injected_ledger") {
		t.Fatalf("expected injected failure, got %v", err)
	}

	var status string
	err = pool.QueryRowContext(ctx, `SELECT status FROM invoices WHERE id = $1 AND organization_id = $2`, inv.ID, orgID).Scan(&status)
	if err != nil {
		t.Fatal(err)
	}
	if status != "draft" {
		t.Fatalf("expected draft after rollback, got %q", status)
	}

	var ledgerCount int
	err = pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE organization_id = $1 AND entity_id = $2`, orgID, inv.ID).Scan(&ledgerCount)
	if err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 0 {
		t.Fatalf("expected no ledger rows for invoice, got %d", ledgerCount)
	}
}
