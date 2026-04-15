package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/httpapi"
)

// Run starts the HTTP server and blocks until context cancellation.
func Run(ctx context.Context, cfg config.Config) error {
	srv := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      httpapi.NewHandler(),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
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
