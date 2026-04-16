package migrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentityMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000002_identity_tenant.up.sql"),
		filepath.Join("000002_identity_tenant.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestAuthSessionsMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000003_auth_sessions.up.sql"),
		filepath.Join("000003_auth_sessions.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestPaymentsMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000011_payments.up.sql"),
		filepath.Join("000011_payments.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestInvoiceGenerationLinkMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000010_invoice_generation_links.up.sql"),
		filepath.Join("000010_invoice_generation_links.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestInvoicesMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000009_invoices.up.sql"),
		filepath.Join("000009_invoices.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestTimeEntryWorkflowMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000008_time_entry_workflow.up.sql"),
		filepath.Join("000008_time_entry_workflow.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestTimeEntriesMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000007_time_entries.up.sql"),
		filepath.Join("000007_time_entries.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestProjectsMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000006_projects.up.sql"),
		filepath.Join("000006_projects.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestClientsMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000005_clients.up.sql"),
		filepath.Join("000005_clients.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}

func TestJobsDeadLetterMigrationFilesExist(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("000004_jobs_dead_letter.up.sql"),
		filepath.Join("000004_jobs_dead_letter.down.sql"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected migration file %s to exist: %v", p, err)
		}
	}
}
