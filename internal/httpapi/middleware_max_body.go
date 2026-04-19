package httpapi

import (
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

func maxBodyMiddleware(cfg config.Config) func(http.Handler) http.Handler {
	limit := cfg.HTTPMaxRequestBodyBytes
	if limit <= 0 {
		limit = 4 << 20
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
			default:
				next.ServeHTTP(w, r)
				return
			}
			if r.ContentLength > limit {
				writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body exceeds maximum allowed size", map[string]any{
					"max_bytes": limit,
				})
				return
			}
			r2 := r.Clone(r.Context())
			r2.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r2)
		})
	}
}
