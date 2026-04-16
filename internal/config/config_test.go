package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_LocalDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	clearOptionalConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "local" {
		t.Fatalf("expected local environment, got %q", cfg.Environment)
	}
	if cfg.HTTP.Addr == "" {
		t.Fatal("expected default HTTP address")
	}
}

func TestLoad_InvalidEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "mars")

	_, err := Load()
	if err == nil {
		t.Fatal("expected invalid APP_ENV error")
	}
	if !strings.Contains(err.Error(), "APP_ENV must be one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_ProductionMissingRequired(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	clearOptionalConfig(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing required vars error")
	}
	if !strings.Contains(err.Error(), "missing required environment variables for production") {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, key := range []string{"DATABASE_URL", "JWT_SIGNING_KEY", "STRIPE_WEBHOOK_SECRET"} {
		if !strings.Contains(err.Error(), key) {
			t.Fatalf("expected missing key %s in error: %v", key, err)
		}
	}
}

func TestLoad_ProductionWithRequired(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected successful config load, got %v", err)
	}
	if cfg.Environment != "production" {
		t.Fatalf("expected production environment, got %q", cfg.Environment)
	}
}

func clearOptionalConfig(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_URL",
		"JWT_SIGNING_KEY",
		"STRIPE_WEBHOOK_SECRET",
		"HTTP_ADDR",
		"HTTP_READ_TIMEOUT_SEC",
		"HTTP_WRITE_TIMEOUT_SEC",
		"HTTP_IDLE_TIMEOUT_SEC",
		"HTTP_SHUTDOWN_TIMEOUT_SEC",
		"INVOICE_PDF_URL_TTL_SEC",
		"PUBLIC_API_BASE_URL",
		"INVOICE_PDF_TOKEN_SECRET",
		"STRIPE_SECRET_KEY",
	} {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
	}
}
