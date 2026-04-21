package seed

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestResetDemoOrganization_BlockedByLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ledger_entries WHERE organization_id = \$1`).
		WithArgs(DemoOrganizationID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	resetErr := ResetDemoOrganization(context.Background(), db)
	if resetErr != ErrLedgerBlocksReset {
		t.Fatalf("expected ErrLedgerBlocksReset, got %v", resetErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeSeedOptions_Defaults(t *testing.T) {
	opts, err := normalizeSeedOptions(SeedOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Preset != PresetHappyPath {
		t.Fatalf("expected default preset %q, got %q", PresetHappyPath, opts.Preset)
	}
	if opts.ContractorCount != 0 {
		t.Fatalf("expected zero contractor count when unspecified, got %d", opts.ContractorCount)
	}
}

func TestNormalizeSeedOptions_ConflictPathDefaultsToSubmitted(t *testing.T) {
	opts, err := normalizeSeedOptions(SeedOptions{
		Preset:          PresetConflictPath,
		ContractorCount: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.SeedSubmittedTime {
		t.Fatal("expected conflict-path preset to default submitted seed")
	}
}

func TestNormalizeSeedOptions_ApprovedImpliesSubmitted(t *testing.T) {
	opts, err := normalizeSeedOptions(SeedOptions{
		Preset:           PresetHappyPath,
		ContractorCount:  1,
		SeedApprovedTime: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.SeedSubmittedTime || !opts.SeedApprovedTime {
		t.Fatalf("expected submitted+approved, got %+v", opts)
	}
}

func TestNormalizeSeedOptions_ContractorRequiredForSubmitted(t *testing.T) {
	_, err := normalizeSeedOptions(SeedOptions{
		Preset:            PresetHappyPath,
		ContractorCount:   0,
		SeedSubmittedTime: true,
	})
	if err == nil {
		t.Fatal("expected error for submitted seed with zero contractors")
	}
}
