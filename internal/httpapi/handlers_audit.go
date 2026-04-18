package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func mountAuditRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/audit-logs", requireTenantAuth(cfg, db, requireRole(authz.ActionAuditRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleListAuditLogs(w, r, db)
	}))))
}

func handleListAuditLogs(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	rows, err := repo.ListAuditLogs(ctx, db, p.OrganizationID, 100)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not list audit logs", nil)
		return
	}

	type outRow struct {
		ID             string         `json:"id"`
		OrganizationID string         `json:"organization_id"`
		ActorUserID    *string        `json:"actor_user_id,omitempty"`
		Action         string         `json:"action"`
		EntityType     string         `json:"entity_type"`
		EntityID       *string        `json:"entity_id,omitempty"`
		Metadata       map[string]any `json:"metadata"`
		CreatedAt      string         `json:"created_at"`
	}
	out := make([]outRow, 0, len(rows))
	for _, row := range rows {
		or := outRow{
			ID:             row.ID.String(),
			OrganizationID: row.OrganizationID.String(),
			Action:         row.Action,
			EntityType:     row.EntityType,
			CreatedAt:      row.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		if row.ActorUserID.Valid {
			s := row.ActorUserID.String
			or.ActorUserID = &s
		}
		if row.EntityID.Valid {
			s := row.EntityID.String
			or.EntityID = &s
		}
		var meta map[string]any
		if len(row.MetadataJSON) > 0 {
			if err := json.Unmarshal(row.MetadataJSON, &meta); err != nil {
				meta = map[string]any{"_parse_error": "invalid metadata_json"}
			}
		} else {
			meta = map[string]any{}
		}
		or.Metadata = meta
		out = append(out, or)
	}
	writeJSON(w, http.StatusOK, out)
}
