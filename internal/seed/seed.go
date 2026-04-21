package seed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

// Deterministic demo identifiers (fake data only).
var (
	DemoOrganizationID = uuid.MustParse("a0000000-0000-4000-8000-000000000001")
	DemoOwnerUserID    = uuid.MustParse("a0000000-0000-4000-8000-000000000002")
)

const (
	demoOrgName      = "Demo Consulting Co"
	demoOwnerEmail   = "owner@demo.local"
	demoOwnerName    = "Demo Owner"
	demoPassword     = "DemoPass1!"
	demoClientName   = "Demo Client LLC"
	demoBillingEmail = "billing@demo-client.local"
	demoProjectName  = "Demo Implementation"
	demoCurrency     = "USD"
)

const (
	PresetMinimal      = "minimal"
	PresetHappyPath    = "happy-path"
	PresetConflictPath = "conflict-path"
)

type SeedOptions struct {
	Preset            string
	ContractorCount   int
	SeedSubmittedTime bool
	SeedApprovedTime  bool
}

// ErrLedgerBlocksReset is returned when demo reset cannot proceed because append-only ledger rows exist for the demo org.
var ErrLedgerBlocksReset = errors.New("cannot reset demo organization: ledger_entries rows exist; remove them manually or use a fresh database")

var errInvalidSeedOptions = errors.New("invalid seed options")

// ResetDemoOrganization removes tenant-scoped demo data so ApplyDemoSeed can be re-run.
// Ledger entries are append-only and are not deleted; reset fails if any exist for the organization.
func ResetDemoOrganization(ctx context.Context, db *sql.DB) error {
	var n int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE organization_id = $1`, DemoOrganizationID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrLedgerBlocksReset
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`DELETE FROM time_entries WHERE organization_id = $1`,
		`DELETE FROM invoices WHERE organization_id = $1`,
		`DELETE FROM invoice_sequences WHERE organization_id = $1`,
		`DELETE FROM projects WHERE organization_id = $1`,
		`DELETE FROM clients WHERE organization_id = $1`,
		`DELETE FROM audit_logs WHERE organization_id = $1`,
		`DELETE FROM memberships WHERE organization_id = $1`,
		`UPDATE organizations SET deactivated_at = NULL, updated_at = NOW() WHERE id = $1`,
	}
	for _, q := range stmts {
		if _, err := tx.ExecContext(ctx, q, DemoOrganizationID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, DemoOwnerUserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE email LIKE 'demo-contractor-%@demo.local'`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, DemoOrganizationID); err != nil {
		return err
	}
	return tx.Commit()
}

// ApplyDemoSeed creates deterministic demo org, owner user, membership, client, project, and a draft time entry.
func ApplyDemoSeed(ctx context.Context, db *sql.DB) error {
	return ApplyDemoSeedWithOptions(ctx, db, SeedOptions{
		Preset:          PresetHappyPath,
		ContractorCount: 1,
	})
}

func ApplyDemoSeedWithOptions(ctx context.Context, db *sql.DB, opts SeedOptions) error {
	opts, err := normalizeSeedOptions(opts)
	if err != nil {
		return err
	}

	hash, err := auth.HashPassword(demoPassword)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO organizations (id, name)
VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  deactivated_at = NULL,
  updated_at = NOW()`, DemoOrganizationID, demoOrgName); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO users (id, email, password_hash, full_name)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  password_hash = EXCLUDED.password_hash,
  full_name = EXCLUDED.full_name,
  updated_at = NOW()`, DemoOwnerUserID, demoOwnerEmail, hash, demoOwnerName); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO memberships (user_id, organization_id, role, status, joined_at)
VALUES ($1, $2, 'owner', 'active', NOW())
ON CONFLICT (user_id, organization_id) DO UPDATE SET
  role = 'owner',
  status = 'active',
  updated_at = NOW()`, DemoOwnerUserID, DemoOrganizationID); err != nil {
		return err
	}

	for i := 1; i <= opts.ContractorCount; i++ {
		contractorID := demoContractorUserID(i)
		contractorEmail := demoContractorEmail(i)
		contractorName := fmt.Sprintf("Demo Contractor %d", i)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO users (id, email, password_hash, full_name)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  password_hash = EXCLUDED.password_hash,
  full_name = EXCLUDED.full_name,
  updated_at = NOW()`, contractorID, contractorEmail, hash, contractorName); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO memberships (user_id, organization_id, role, status, joined_at)
VALUES ($1, $2, 'contractor', 'active', NOW())
ON CONFLICT (user_id, organization_id) DO UPDATE SET
  role = 'contractor',
  status = 'active',
  updated_at = NOW()`, contractorID, DemoOrganizationID); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if opts.Preset == PresetMinimal {
		return nil
	}

	var existingClients int
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM clients WHERE organization_id = $1 AND deleted_at IS NULL AND name = $2`,
		DemoOrganizationID, demoClientName).Scan(&existingClients); err != nil {
		return err
	}
	if existingClients > 0 {
		return nil
	}

	clientID, err := repo.CreateClient(ctx, db, DemoOrganizationID, demoClientName, demoBillingEmail, demoCurrency)
	if err != nil {
		return fmt.Errorf("create demo client: %w", err)
	}
	projectID, err := repo.CreateProject(ctx, db, DemoOrganizationID, clientID, demoProjectName, "hourly", 15000)
	if err != nil {
		return fmt.Errorf("create demo project: %w", err)
	}
	workDate := time.Now().UTC().Truncate(24 * time.Hour)
	notes := "Seeded draft entry"
	entryUserID := DemoOwnerUserID
	if opts.ContractorCount > 0 {
		entryUserID = demoContractorUserID(1)
		notes = "Seeded contractor entry"
	}
	entryID, err := repo.CreateTimeEntry(ctx, db, DemoOrganizationID, projectID, entryUserID, workDate, 60, 15000, &notes)
	if err != nil {
		return fmt.Errorf("create demo time entry: %w", err)
	}

	if opts.SeedSubmittedTime || opts.SeedApprovedTime {
		if err := repo.SubmitTimeEntry(ctx, db, DemoOrganizationID, entryID, entryUserID); err != nil {
			return fmt.Errorf("submit seeded time entry: %w", err)
		}
	}
	if opts.SeedApprovedTime {
		if err := repo.ApproveTimeEntry(ctx, db, DemoOrganizationID, entryID, DemoOwnerUserID); err != nil {
			return fmt.Errorf("approve seeded time entry: %w", err)
		}
	}
	return nil
}

func normalizeSeedOptions(opts SeedOptions) (SeedOptions, error) {
	out := opts
	out.Preset = strings.TrimSpace(strings.ToLower(out.Preset))
	if out.Preset == "" {
		out.Preset = PresetHappyPath
	}
	if out.ContractorCount < 0 {
		return SeedOptions{}, fmt.Errorf("%w: contractor count must be >= 0", errInvalidSeedOptions)
	}
	if out.ContractorCount == 0 && out.SeedSubmittedTime {
		return SeedOptions{}, fmt.Errorf("%w: submitted/approved seeded time requires at least one contractor", errInvalidSeedOptions)
	}
	if out.SeedApprovedTime {
		out.SeedSubmittedTime = true
	}

	switch out.Preset {
	case PresetMinimal:
		out.SeedSubmittedTime = false
		out.SeedApprovedTime = false
	case PresetHappyPath:
		// Defaults intentionally keep draft entry unless toggles are explicitly set.
	case PresetConflictPath:
		if !opts.SeedSubmittedTime && !opts.SeedApprovedTime {
			out.SeedSubmittedTime = true
		}
	default:
		return SeedOptions{}, fmt.Errorf("%w: unsupported preset %q", errInvalidSeedOptions, out.Preset)
	}

	return out, nil
}

func demoContractorEmail(i int) string {
	return fmt.Sprintf("demo-contractor-%d@demo.local", i)
}

func demoContractorUserID(i int) uuid.UUID {
	return uuid.NewSHA1(DemoOrganizationID, []byte(fmt.Sprintf("demo-contractor-%d", i)))
}
