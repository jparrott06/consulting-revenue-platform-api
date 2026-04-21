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
	preset := flag.String("preset", seed.PresetHappyPath, "seed preset: minimal|happy-path|conflict-path")
	contractors := flag.Int("contractors", 1, "number of deterministic contractor users to seed")
	seedSubmitted := flag.Bool("seed-submitted-time", false, "pre-stage seeded time entry in submitted state")
	seedApproved := flag.Bool("seed-approved-time", false, "pre-stage seeded time entry in approved state (implies submitted)")
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

	opts := seed.SeedOptions{
		Preset:            *preset,
		ContractorCount:   *contractors,
		SeedSubmittedTime: *seedSubmitted,
		SeedApprovedTime:  *seedApproved,
	}
	if err := seed.ApplyDemoSeedWithOptions(ctx, pool, opts); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("demo seed applied (org=%s owner=%s login=%s password=%s preset=%s contractors=%d submitted=%t approved=%t)",
		seed.DemoOrganizationID, seed.DemoOwnerUserID, "owner@demo.local", "DemoPass1!", *preset, *contractors, *seedSubmitted || *seedApproved, *seedApproved)
	os.Exit(0)
}
