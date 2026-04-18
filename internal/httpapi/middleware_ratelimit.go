package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"golang.org/x/time/rate"
)

func rateLimitMiddleware(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		var mu sync.Mutex
		limiterByKey := make(map[string]*rate.Limiter)

		getLimiter := func(key string, perMin int) *rate.Limiter {
			mu.Lock()
			defer mu.Unlock()
			lim, ok := limiterByKey[key]
			if ok {
				return lim
			}
			if perMin < 1 {
				perMin = 1
			}
			lim = rate.NewLimiter(rate.Limit(float64(perMin)/60.0), perMin)
			limiterByKey[key] = lim
			return lim
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkipRateLimit(r) || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)
			cat := rateLimitCategory(r.URL.Path)
			var perMin int
			switch cat {
			case "auth":
				perMin = cfg.RateLimitAuthPerMinute
			case "webhook":
				perMin = cfg.RateLimitWebhookPerMinute
			default:
				perMin = cfg.RateLimitDefaultPerMinute
			}

			lim := getLimiter(cat+":"+ip, perMin)
			if !lim.Allow() {
				w.Header().Set("Retry-After", strconv.Itoa(60))
				writeError(r.Context(), w, http.StatusTooManyRequests, "rate_limited", "too many requests", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func shouldSkipRateLimit(r *http.Request) bool {
	switch r.URL.Path {
	case "/healthz", "/livez", "/readyz", "/metrics":
		return true
	default:
		return false
	}
}

func rateLimitCategory(path string) string {
	switch {
	case strings.HasPrefix(path, "/auth/login"),
		strings.HasPrefix(path, "/auth/register"),
		strings.HasPrefix(path, "/auth/refresh"):
		return "auth"
	case strings.HasPrefix(path, "/webhooks/"):
		return "webhook"
	default:
		return "default"
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
