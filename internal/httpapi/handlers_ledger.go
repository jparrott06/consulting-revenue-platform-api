package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func mountLedgerRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/ledger", requireTenantAuth(cfg, db, requireRole(authz.ActionLedgerRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleListLedger(w, r, db)
	}))))
}

func handleListLedger(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	if db == nil {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
		return
	}
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}

	limit := 50
	if l := strings.TrimSpace(r.URL.Query().Get("limit")); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 200 {
		limit = 200
	}

	entries, err := repo.ListLedgerEntries(ctx, db, p.OrganizationID, limit)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not list ledger entries", nil)
		return
	}

	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		var meta any
		if len(e.MetadataJSON) > 0 {
			if err := json.Unmarshal(e.MetadataJSON, &meta); err != nil {
				meta = map[string]any{}
			}
		} else {
			meta = map[string]any{}
		}
		out = append(out, map[string]any{
			"id":              e.ID.String(),
			"event_type":      e.EventType,
			"entity_type":     e.EntityType,
			"entity_id":       e.EntityID.String(),
			"amount_minor":    e.AmountMinor,
			"currency":        e.Currency,
			"metadata":        meta,
			"created_at":      e.CreatedAt.UTC().Format(time.RFC3339Nano),
			"organization_id": e.OrganizationID.String(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}
