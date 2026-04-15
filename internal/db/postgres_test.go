package db

import (
	"context"
	"strings"
	"testing"
)

func TestOpenPostgres_RequiresDatabaseURL(t *testing.T) {
	_, err := OpenPostgres(context.Background(), "")
	if err == nil {
		t.Fatal("expected missing DATABASE_URL error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
