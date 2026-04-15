package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		direction = flag.String("direction", "up", "migration direction: up or down")
		path      = flag.String("path", "migrations", "path to migration files")
	)
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	sourceURL := fmt.Sprintf("file://%s", *path)
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		log.Fatalf("create migration runner: %v", err)
	}

	switch *direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	default:
		log.Fatalf("invalid direction %q: expected up or down", *direction)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("run migration %s: %v", *direction, err)
	}

	log.Printf("migration %s completed", *direction)
}
