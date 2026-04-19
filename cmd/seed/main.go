package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/db"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/seed"
)

func main() {
	reset := flag.Bool("reset", false, "remove demo organization data before seeding (fails if ledger_entries exist for the demo org)")
	flag.Parse()

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

	if *reset {
		if err := seed.ResetDemoOrganization(ctx, pool); err != nil {
			log.Fatalf("reset demo: %v", err)
		}
		log.Printf("reset demo organization %s", seed.DemoOrganizationID)
	}

	if err := seed.ApplyDemoSeed(ctx, pool); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("demo seed applied (org=%s user=%s login=%s password=%s)",
		seed.DemoOrganizationID, seed.DemoOwnerUserID, "owner@demo.local", "DemoPass1!")
	os.Exit(0)
}
