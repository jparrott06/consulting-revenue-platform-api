package httpapi

import (
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
)

func meHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"user_id":         p.UserID.String(),
		"organization_id": p.OrganizationID.String(),
		"role":            p.Role,
	})
}

func adminPingHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"scope":  authz.ActionAdminOps,
	})
}
