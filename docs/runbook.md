# Operations runbook

This runbook covers day-to-day operation of the Consulting Revenue Platform API: configuration, database migrations, background jobs, and basic incident checks.

## Prerequisites

- Go toolchain matching `go.mod`.
- PostgreSQL 16+ (local or managed).
- Environment variables (see repository `.env.example`).

## Configuration and secrets

- Never commit secrets. Use environment variables or a secret manager.
- Production requires `DATABASE_URL`, `JWT_SIGNING_KEY`, and `STRIPE_WEBHOOK_SECRET` (see `internal/config` validation).
- Optional operational toggles:
  - `WEBHOOK_WORKER_ENABLED` — processes Stripe webhook events from the database queue.
  - `RETENTION_WORKER_ENABLED` — periodically purges old `audit_logs` and `webhook_events` rows per configured windows.
- See [threat-model.md](threat-model.md) for security controls mapping.

## Database migrations

Apply schema changes:

```bash
export DATABASE_URL='postgres://...'
make migrate-up
```

Rollback one step (use with care):

```bash
make migrate-down
```

CI runs `go run ./cmd/migrate -direction up` against a clean Postgres instance on every change.

## Demo and local data

Deterministic demo tenant (fake data only):

```bash
export DATABASE_URL='postgres://...'
make migrate-up
go run ./cmd/seed --reset   # optional: clears demo org scope (fails if ledger_entries exist for demo org)
make seed                    # or: go run ./cmd/seed
```

Login: `owner@demo.local` / `DemoPass1!` (see `cmd/seed` log line). Organization ID is fixed in `internal/seed` for repeatable demos.

## Operational retention

Purge **eligible** operational rows older than configured windows (does not touch invoices, ledger, payments, or business entities):

- `RETENTION_AUDIT_LOG_DAYS` (default 365, clamped 30–3650)
- `RETENTION_WEBHOOK_EVENT_DAYS` (default 90, clamped 7–730)

One-shot (suitable for cron):

```bash
go run ./cmd/retention
```

In-process worker (same process as API):

```bash
export RETENTION_WORKER_ENABLED=true
export RETENTION_WORKER_POLL_INTERVAL_SEC=3600
go run ./cmd/api
```

## Organization deactivation

- `POST /v1/organizations/{organization_id}/deactivate` (owner only; path UUID must match `X-Organization-ID`).
- Sets `organizations.deactivated_at` and suspends all active memberships. Financial tables are not deleted.
- After deactivation, tenant APIs return **403** for that organization because memberships are no longer active.

## Health and observability

- `GET /healthz`, `GET /livez` — process liveness.
- `GET /readyz` — database connectivity.
- `GET /metrics` — Prometheus metrics (internal scrape; not in public OpenAPI surface).

### Prometheus (RED-style HTTP metrics)

Counters and histograms use **low-cardinality** labels only (`method`, `route`, `status_class` such as `2xx`/`4xx`/`5xx`). Business workflow conflicts are counted separately:

- `http_requests_total{method,route,status_class}` — request rate and error class.
- `http_request_duration_seconds{method,route}` — latency.
- `business_workflow_conflict_total{domain,action}` — 409 outcomes on invoice and time-entry workflows.

Example queries:

```promql
# 5xx rate for a route pattern
sum(rate(http_requests_total{route="POST /v1/invoices/generate",status_class="5xx"}[5m]))

# p95 latency
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{route="POST /v1/invoices/generate"}[5m])) by (le))

# Workflow conflicts
sum(rate(business_workflow_conflict_total[5m])) by (domain, action)
```

Structured log fields for HTTP are documented in [logging.md](logging.md). Transaction ownership for critical routes is in [transaction-matrix.md](transaction-matrix.md).

### Stripe webhooks

- Event type → handler mapping: [stripe-webhook-matrix.md](stripe-webhook-matrix.md).
- Worker metric: `stripe_webhook_worker_outcomes_total{event_category,outcome}` (e.g. `outcome="success"` vs `retry` vs `terminal`).

```promql
sum(rate(stripe_webhook_worker_outcomes_total{outcome="retry"}[15m])) by (event_category)
```

### Reconciliation (ledger vs paid invoices)

- **Endpoint:** `GET /v1/admin/reconciliation-summary` (owner / `admin.ops` only; same auth as `GET /v1/admin/ping`).
- **Meaning:** For each currency, compares the sum of ledger `payment_captured` rows to the sum of `payments` rows tied to invoices in `paid` status. **`drift_minor`** is ledger minus paid-invoice payments; **`aligned`** is true when `drift_minor` is zero.
- **When drift is non-zero:** Confirm Stripe webhook delivery and worker processing (`webhook_events`, worker logs). Check for manual DB edits, duplicate captures, or invoices marked paid without a matching ledger append (see [transaction-matrix.md](transaction-matrix.md)). This check is read-only; it does not fix data or call Stripe.

## Incident basics

1. Confirm Postgres reachable (`readyz`).
2. Check application logs for `panic recovered`, `webhookworker`, or `retention` messages (JSON fields include `request_id` / `correlation_id` where applicable).
3. Verify migrations applied on the target database version.
4. For Stripe issues, confirm webhook secret matches the Stripe dashboard endpoint and that events are persisted (`webhook_events`).

### Transaction rollback or stuck workflows

1. Correlate failing requests using `request_id` from API error JSON and access logs (`http_request` lines).
2. Inspect `business_workflow_conflict_total` and `http_requests_total` for spikes on `POST /v1/invoices/*` or `POST /v1/time-entries/*`.
3. For suspected partial writes, confirm DB invariants: invoice generation and send are single transactions per [transaction-matrix.md](transaction-matrix.md); integration tests in `internal/integrationtest/rollback_test.go` document expected rollback behavior.
4. For stuck time entries, verify row status in `time_entries` and recent `time_entry_events` for the tenant.

## Local PR verification (operability)

Before merging changes that touch workflows, metrics, or logging:

- `gofmt ./...`, `go test ./...`, `go test -race ./...`
- With Postgres migrated: `DATABASE_URL=... go test ./internal/integrationtest/...` (rollback tests exercise real transactions).

## API contract

- OpenAPI: [openapi.yaml](openapi.yaml) (validated in CI).

## References

- [Threat model](threat-model.md)
- [README](../README.md) — contributor setup and architecture overview
