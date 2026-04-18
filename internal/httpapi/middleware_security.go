package httpapi

import (
	"net/http"
	"strings"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if r.Method == http.MethodOptions {
				if origin != "" && corsOriginOK(cfg, origin) {
					setCORSHeaders(w, origin)
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if origin != "" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if origin != "" && corsOriginOK(cfg, origin) {
				setCORSHeaders(w, origin)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Organization-ID, X-Request-ID")
	w.Header().Set("Access-Control-Max-Age", "7200")
}

func corsOriginOK(cfg config.Config, origin string) bool {
	if origin == "" {
		return false
	}
	for _, o := range cfg.CORSAllowedOrigins {
		if o == origin {
			return true
		}
	}
	if len(cfg.CORSAllowedOrigins) == 0 && (cfg.Environment == "local" || cfg.Environment == "development") {
		return strings.HasPrefix(origin, "http://localhost:") ||
			strings.HasPrefix(origin, "http://127.0.0.1:")
	}
	return false
}
