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

## Incident basics

1. Confirm Postgres reachable (`readyz`).
2. Check application logs for `panic recovered`, `webhookworker`, or `retention` messages.
3. Verify migrations applied on the target database version.
4. For Stripe issues, confirm webhook secret matches the Stripe dashboard endpoint and that events are persisted (`webhook_events`).

## API contract

- OpenAPI: [openapi.yaml](openapi.yaml) (validated in CI).

## References

- [Threat model](threat-model.md)
- [README](../README.md) — contributor setup and architecture overview
