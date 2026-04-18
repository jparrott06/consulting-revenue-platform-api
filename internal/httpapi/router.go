package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

// NewHandler returns the root HTTP router for the API.
func NewHandler(cfg config.Config, db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	login, refresh, logout := authHandlers(cfg, db)
	mux.HandleFunc("GET /healthz", healthHandler)
	mountStripeWebhookRoute(mux, cfg, db)
	mux.HandleFunc("POST /auth/register", registerHandler(db))
	mux.HandleFunc("POST /auth/login", login)
	mux.HandleFunc("POST /auth/refresh", refresh)
	mux.HandleFunc("POST /auth/logout", logout)

	mux.Handle("GET /v1/me", requireTenantAuth(cfg, db, requireRole(authz.ActionContextRead, http.HandlerFunc(meHandler))))
	mux.Handle("GET /v1/admin/ping", requireTenantAuth(cfg, db, requireRole(authz.ActionAdminOps, http.HandlerFunc(adminPingHandler))))

	mountMembershipRoutes(mux, cfg, db)
	mountClientRoutes(mux, cfg, db)
	mountProjectRoutes(mux, cfg, db)
	mountTimeEntryRoutes(mux, cfg, db)
	mountTimeEntryWorkflowRoutes(mux, cfg, db)
	mountInvoiceRoutes(mux, cfg, db)
	mountLedgerRoutes(mux, cfg, db)
	mountPublicDocumentRoutes(mux, cfg, db)

	return chain(
		mux,
		requestIDMiddleware,
		recoveryMiddleware,
		timeoutMiddleware,
		loggingMiddleware,
	)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
