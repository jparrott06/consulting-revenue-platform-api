# Structured logging conventions

Logs intended for production aggregation should use **stable, machine-oriented field names** and avoid embedding secrets or tenant identifiers in labels.

## HTTP requests

Emitted by `internal/httpapi` observability middleware for each request:

| Field | Description |
|-------|-------------|
| `request_id` | Correlation ID from `X-Request-ID` or generated (also returned in JSON error bodies). |
| `method` | HTTP method. |
| `path` | Path with sensitive query strings redacted (`internal/logredact`). |
| `route` | Go 1.22 route pattern (e.g. `GET /v1/me`) or `unmatched`. |
| `status` | HTTP status code. |
| `duration_ms` | Wall time for the request. |
| `bytes` | Response bytes written. |

Panic recovery logs (`panic recovered`) include `request_id`, `method`, `path`, and `route`.

## Background workers

| Field | Description |
|-------|-------------|
| `component` | `webhookworker` or `retention`. |
| `correlation_id` | Random hex ID for a single poll/processing attempt (not an org or user id). |
| `msg` | Short human-readable event (e.g. process error, rows removed). |

## Prohibited in structured fields

- Authorization headers, bearer tokens, Stripe secrets, webhook signing secrets.
- Raw payment payloads or full request bodies on error paths.

Existing redaction helpers in `internal/logredact` remain authoritative for URLs and shared log lines.
