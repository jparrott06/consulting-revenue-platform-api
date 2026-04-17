package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/db"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/httpapi"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/webhookworker"
)

// Run starts the HTTP server and blocks until context cancellation.
func Run(ctx context.Context, cfg config.Config) error {
	var pool *sql.DB
	if cfg.DatabaseURL != "" {
		conn, err := db.OpenPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("database connectivity check failed: %w", err)
		}
		defer func() { _ = conn.Close() }()
		pool = conn
	}

	srv := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      httpapi.NewHandler(cfg, pool),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	if pool != nil && cfg.WebhookWorkerEnabled {
		go func() {
			if err := webhookworker.Run(ctx, cfg, pool); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("webhook worker exited: %v", err)
			}
		}()
	}

	errCh := make(chan error, 1)

	go func() {
		log.Printf("api listening on %s", cfg.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server error: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown failed: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}
