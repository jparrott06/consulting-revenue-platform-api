package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/logredact"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed (RED rate/errors; labels are low-cardinality).",
		},
		[]string{"method", "route", "status_class"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds (RED duration).",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)

	accessLogger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
)

func observabilityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		httpRequestsTotal.WithLabelValues(r.Method, route, statusClass(rec.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(started).Seconds())

		logPath, logQuery := logredact.SanitizeURL(r.URL)
		args := []any{
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", logPath,
			"route", route,
			"status", rec.statusCode,
			"duration_ms", time.Since(started).Milliseconds(),
			"bytes", rec.bytesWritten,
		}
		if logQuery != "" {
			args = append(args, "query", logQuery)
		}
		accessLogger.Info("http_request", args...)
	})
}

func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "other"
	}
}
