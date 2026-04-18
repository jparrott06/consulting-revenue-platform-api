package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"
)

const probeDBTimeout = 2 * time.Second

func liveHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			writeProbeNotReady(w)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), probeDBTimeout)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			writeProbeNotReady(w)
			return
		}
		var one int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM webhook_events LIMIT 1`).Scan(&one)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeProbeNotReady(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeProbeNotReady(w http.ResponseWriter) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
}
