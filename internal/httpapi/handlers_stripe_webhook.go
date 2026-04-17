package httpapi

import (
	"database/sql"
	"io"
	"net/http"
	"strings"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
	"github.com/stripe/stripe-go/v81/webhook"
)

const maxStripeWebhookBodyBytes = 1 << 20

func mountStripeWebhookRoute(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.HandleFunc("POST /webhooks/stripe", handleStripeWebhook(cfg, db))
}

func handleStripeWebhook(cfg config.Config, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if db == nil {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
			return
		}
		secret := strings.TrimSpace(cfg.StripeWebhookSecret)
		if secret == "" {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "Stripe webhook secret is not configured", nil)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxStripeWebhookBodyBytes)
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "could not read request body", nil)
			return
		}

		sig := strings.TrimSpace(r.Header.Get("Stripe-Signature"))
		if sig == "" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "Stripe-Signature header is required", nil)
			return
		}

		event, err := webhook.ConstructEvent(payload, sig, secret)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid webhook signature", nil)
			return
		}

		inserted, err := repo.InsertStripeWebhookEvent(ctx, db, event.ID, string(event.Type), payload)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not persist webhook event", nil)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"received": true,
			"inserted": inserted,
		})
	}
}
