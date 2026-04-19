package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEnvironment     = "local"
	defaultHTTPAddr        = ":8080"
	defaultReadTimeoutSec  = 10
	defaultWriteTimeoutSec = 10
	defaultIdleTimeoutSec  = 60
	defaultMaxRequestBody  = 4 << 20
)

var validEnvironments = map[string]struct{}{
	"local":       {},
	"development": {},
	"staging":     {},
	"production":  {},
}

// HTTPConfig controls network binding and server timeout behavior.
type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Config is the root app configuration.
type Config struct {
	Environment                 string
	DatabaseURL                 string
	CORSAllowedOrigins          []string
	RateLimitAuthPerMinute      int
	RateLimitDefaultPerMinute   int
	RateLimitWebhookPerMinute   int
	JWTSigningKey               string
	JWTAccessTTL                time.Duration
	JWTRefreshTTL               time.Duration
	StripeWebhookSecret         string
	StripeSecretKey             string
	InvoicePDFURLTTL            time.Duration
	PublicAPIBaseURL            string
	InvoicePDFTokenSecret       string
	WebhookWorkerEnabled        bool
	WebhookWorkerPollInterval   time.Duration
	WebhookWorkerMaxAttempts    int
	HTTPMaxRequestBodyBytes     int64
	RetentionAuditLogDays       int
	RetentionWebhookEventDays   int
	RetentionWorkerEnabled      bool
	RetentionWorkerPollInterval time.Duration
	HTTP                        HTTPConfig
}

// Load returns configuration from env vars with safe defaults for local dev.
func Load() (Config, error) {
	environment := stringFromEnv("APP_ENV", defaultEnvironment)
	if !isValidEnvironment(environment) {
		return Config{}, fmt.Errorf("APP_ENV must be one of local, development, staging, production")
	}

	readTimeoutSec, err := intFromEnv("HTTP_READ_TIMEOUT_SEC", defaultReadTimeoutSec)
	if err != nil {
		return Config{}, err
	}
	writeTimeoutSec, err := intFromEnv("HTTP_WRITE_TIMEOUT_SEC", defaultWriteTimeoutSec)
	if err != nil {
		return Config{}, err
	}
	idleTimeoutSec, err := intFromEnv("HTTP_IDLE_TIMEOUT_SEC", defaultIdleTimeoutSec)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeoutSec, err := intFromEnv("HTTP_SHUTDOWN_TIMEOUT_SEC", 10)
	if err != nil {
		return Config{}, err
	}

	accessMin, err := intFromEnv("JWT_ACCESS_TTL_MIN", 15)
	if err != nil {
		return Config{}, err
	}
	refreshDays, err := intFromEnv("JWT_REFRESH_TTL_DAYS", 7)
	if err != nil {
		return Config{}, err
	}

	pdfURLTTLSec, err := intFromEnv("INVOICE_PDF_URL_TTL_SEC", 300)
	if err != nil {
		return Config{}, err
	}
	if pdfURLTTLSec < 60 {
		pdfURLTTLSec = 60
	}
	if pdfURLTTLSec > 3600 {
		pdfURLTTLSec = 3600
	}

	webhookPollSec, err := intFromEnv("WEBHOOK_WORKER_POLL_INTERVAL_SEC", 5)
	if err != nil {
		return Config{}, err
	}
	if webhookPollSec < 1 {
		webhookPollSec = 1
	}
	if webhookPollSec > 300 {
		webhookPollSec = 300
	}

	webhookMaxAttempts, err := intFromEnv("WEBHOOK_WORKER_MAX_ATTEMPTS", 5)
	if err != nil {
		return Config{}, err
	}
	if webhookMaxAttempts < 2 {
		webhookMaxAttempts = 2
	}

	rateAuth, err := intFromEnv("RATE_LIMIT_AUTH_PER_MIN", 30)
	if err != nil {
		return Config{}, err
	}
	rateDef, err := intFromEnv("RATE_LIMIT_DEFAULT_PER_MIN", 200)
	if err != nil {
		return Config{}, err
	}
	rateWH, err := intFromEnv("RATE_LIMIT_WEBHOOK_PER_MIN", 120)
	if err != nil {
		return Config{}, err
	}

	corsOrigins := parseOriginList(strings.TrimSpace(stringFromEnv("CORS_ALLOWED_ORIGINS", "")))

	retentionAuditDays, err := intFromEnv("RETENTION_AUDIT_LOG_DAYS", 365)
	if err != nil {
		return Config{}, err
	}
	if retentionAuditDays < 30 {
		retentionAuditDays = 30
	}
	if retentionAuditDays > 3650 {
		retentionAuditDays = 3650
	}

	retentionWebhookDays, err := intFromEnv("RETENTION_WEBHOOK_EVENT_DAYS", 90)
	if err != nil {
		return Config{}, err
	}
	if retentionWebhookDays < 7 {
		retentionWebhookDays = 7
	}
	if retentionWebhookDays > 730 {
		retentionWebhookDays = 730
	}

	retentionPollSec, err := intFromEnv("RETENTION_WORKER_POLL_INTERVAL_SEC", 3600)
	if err != nil {
		return Config{}, err
	}
	if retentionPollSec < 60 {
		retentionPollSec = 60
	}
	if retentionPollSec > 86400 {
		retentionPollSec = 86400
	}

	maxBodyBytes := int64(defaultMaxRequestBody)
	if raw := strings.TrimSpace(os.Getenv("HTTP_MAX_REQUEST_BODY_BYTES")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("HTTP_MAX_REQUEST_BODY_BYTES must be an integer: %w", err)
		}
		const minBody = 4096
		if v < minBody {
			return Config{}, fmt.Errorf("HTTP_MAX_REQUEST_BODY_BYTES must be at least %d", minBody)
		}
		maxBodyBytes = v
	}

	cfg := Config{
		Environment:                 environment,
		DatabaseURL:                 stringFromEnv("DATABASE_URL", ""),
		CORSAllowedOrigins:          corsOrigins,
		RateLimitAuthPerMinute:      rateAuth,
		RateLimitDefaultPerMinute:   rateDef,
		RateLimitWebhookPerMinute:   rateWH,
		JWTSigningKey:               stringFromEnv("JWT_SIGNING_KEY", ""),
		JWTAccessTTL:                time.Duration(accessMin) * time.Minute,
		JWTRefreshTTL:               time.Duration(refreshDays) * 24 * time.Hour,
		StripeWebhookSecret:         stringFromEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripeSecretKey:             stringFromEnv("STRIPE_SECRET_KEY", ""),
		InvoicePDFURLTTL:            time.Duration(pdfURLTTLSec) * time.Second,
		PublicAPIBaseURL:            strings.TrimRight(strings.TrimSpace(stringFromEnv("PUBLIC_API_BASE_URL", "")), "/"),
		InvoicePDFTokenSecret:       stringFromEnv("INVOICE_PDF_TOKEN_SECRET", ""),
		WebhookWorkerEnabled:        boolFromEnv("WEBHOOK_WORKER_ENABLED", false),
		WebhookWorkerPollInterval:   time.Duration(webhookPollSec) * time.Second,
		WebhookWorkerMaxAttempts:    webhookMaxAttempts,
		HTTPMaxRequestBodyBytes:     maxBodyBytes,
		RetentionAuditLogDays:       retentionAuditDays,
		RetentionWebhookEventDays:   retentionWebhookDays,
		RetentionWorkerEnabled:      boolFromEnv("RETENTION_WORKER_ENABLED", false),
		RetentionWorkerPollInterval: time.Duration(retentionPollSec) * time.Second,
		HTTP: HTTPConfig{
			Addr:            stringFromEnv("HTTP_ADDR", defaultHTTPAddr),
			ReadTimeout:     time.Duration(readTimeoutSec) * time.Second,
			WriteTimeout:    time.Duration(writeTimeoutSec) * time.Second,
			IdleTimeout:     time.Duration(idleTimeoutSec) * time.Second,
			ShutdownTimeout: time.Duration(shutdownTimeoutSec) * time.Second,
		},
	}

	if err := validateRequired(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func intFromEnv(key string, fallback int) (int, error) {
	raw := stringFromEnv(key, "")
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}

	return value, nil
}

func stringFromEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func boolFromEnv(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func isValidEnvironment(value string) bool {
	_, ok := validEnvironments[value]
	return ok
}

func parseOriginList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateRequired(cfg Config) error {
	if cfg.Environment != "production" {
		return nil
	}

	missing := make([]string, 0, 3)
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWTSigningKey == "" {
		missing = append(missing, "JWT_SIGNING_KEY")
	}
	if cfg.StripeWebhookSecret == "" {
		missing = append(missing, "STRIPE_WEBHOOK_SECRET")
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing required environment variables for production: %s", strings.Join(missing, ", "))
	}

	return nil
}
