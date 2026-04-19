package main

import (
	"context"
	"log"
	"os"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/db"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/retention"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := db.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer func() { _ = pool.Close() }()

	a, w, err := retention.RunOnce(ctx, pool, cfg)
	if err != nil {
		log.Fatalf("retention: %v", err)
	}
	log.Printf("retention completed audit_logs=%d webhook_events=%d", a, w)
	os.Exit(0)
}
