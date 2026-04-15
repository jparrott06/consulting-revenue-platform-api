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
