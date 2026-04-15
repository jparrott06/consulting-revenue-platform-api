package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/app"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := app.Run(ctx, cfg); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
