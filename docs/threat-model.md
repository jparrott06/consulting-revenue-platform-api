# Threat model and security controls (V1)

This document summarizes primary abuse scenarios for the Consulting Revenue Platform API, how the implementation mitigates them, and where automated tests or operational practices provide assurance. It is a living engineering document, not a formal third-party audit.

## Scope and assumptions

- Multi-tenant SaaS API with shared database and strict `organization_id` scoping.
- Authenticated users hold organization-scoped roles (`owner`, `accountant`, `contractor`).
- Secrets and keys are supplied only via environment variables or a secret manager; they are never logged.

## Data classification (logging and audits)

| Class | Examples | Logging policy |
| ----- | -------- | ---------------- |
| Secret | Passwords, refresh tokens, JWT signing keys, Stripe webhook secrets, API keys | Never written to logs or audit `metadata` in clear form; query strings containing sensitive keys are redacted in access logs (`internal/logredact`). |
| Sensitive personal | Email addresses, full names | Allowed in application-controlled audit metadata where required for accountability; avoid in noisy debug logs. |
| Operational | HTTP method, path, status, duration, request ID | Logged in structured access logs. |

## Threat catalog

### 1. Tenant breakout (IDOR / cross-tenant data access)

**Scenario:** An attacker with a valid token for organization A attempts to read or mutate resources belonging to organization B by guessing or obtaining UUIDs.

**Controls:** Every tenant-scoped repository call includes `organization_id` in the `WHERE` clause. HTTP handlers resolve the active organization from `X-Organization-ID` plus membership, not from client-supplied entity payloads.

**Tests:** `internal/httpapi/authz_regression_test.go` (cross-tenant `GET`/`PATCH` client and invoice PDF paths expect `404` / `not_found` when the row is absent for the caller organization).

**Residual risk:** Logical bugs in new endpoints that bypass repository scoping. Mitigation: code review checklist and expanding the regression suite as new resource types are added.

### 2. Broken access control (role escalation)

**Scenario:** A `contractor` invokes owner-only or accountant-only actions (for example, invoice generation or membership administration).

**Controls:** Central RBAC in `internal/authz/policy.go` combined with `requireRole` on routes.

**Tests:** Existing `*_authz_test.go` files under `internal/httpapi` (for example `clients_authz_test.go`, `invoices_authz_test.go`).

**Residual risk:** New actions added without a matching `requireRole` guard. Mitigation: default-deny mindset and authz tests per route group.

### 3. Authentication and session abuse

**Scenario:** Token theft, refresh token reuse, or session fixation.

**Controls:** Short-lived access JWTs, hashed refresh tokens in the database, refresh rotation, and password hashing via `internal/auth`. Global request body limits reduce credential stuffing payload sizes.

**Tests:** Session and auth handler tests in `internal/httpapi` and `internal/repo` (where applicable).

**Residual risk:** Compromised end-user devices. Mitigation: TLS everywhere, future device binding if required.

### 4. Webhook spoofing and replay (Stripe)

**Scenario:** An attacker forges Stripe events or replays old events to manipulate payment state.

**Controls:** Stripe signature verification (`webhook.ConstructEvent`), idempotent event persistence, and bounded webhook body size.

**Tests:** `internal/httpapi/handlers_stripe_webhook_test.go` and reconciliation tests under `internal/repo` / worker packages.

**Residual risk:** Leaked webhook signing secret. Mitigation: secret rotation and monitoring for anomalous event volume.

### 5. SQL injection

**Scenario:** Attacker-controlled strings concatenated into SQL.

**Controls:** Parameterized queries throughout `internal/repo`.

**Tests:** Indirectly covered by integration-style tests using real query shapes; prefer static analysis (`go vet`, SQL review) for new queries.

**Residual risk:** Unsafe dynamic SQL introduced in future changes. Mitigation: linting and review standards.

### 6. CSV injection (exports)

**Scenario:** Malicious text in stored names or descriptions becomes a formula when opened in Excel.

**Controls:** Export pipelines prefix risky cell-leading characters (see export handlers and related tests).

**Tests:** `internal/httpapi/exports_authz_test.go` and CSV-focused tests where present.

**Residual risk:** New export formats without sanitization. Mitigation: reuse shared sanitization helpers.

### 7. Denial of service and oversized payloads

**Scenario:** Huge JSON bodies exhaust memory or CPU.

**Controls:** Configurable `HTTP_MAX_REQUEST_BODY_BYTES` (default 4 MiB), `http.MaxBytesReader` in middleware for mutating methods, strict JSON decoding with unknown-field rejection, and per-route validation.

**Tests:** `internal/httpapi/middleware_max_body_test.go`, `internal/httpapi/decode_test.go`, and `internal/config` tests for the env knob.

**Residual risk:** Application-level algorithmic complexity (for example, pathological reports). Mitigation: pagination, timeouts (`timeoutMiddleware`), and rate limits.

### 8. Information disclosure via logs

**Scenario:** Passwords, tokens, or webhook secrets appear in access logs or panic traces.

**Controls:** Structured logging with `internal/logredact` for query parameters; panic responses sanitized; audit metadata passed through `repo.RedactAuditMetadata` where applicable.

**Tests:** `internal/logredact/logredact_test.go`; `internal/httpapi/router_test.go` (panic body does not leak details).

**Residual risk:** New log fields added without classification. Mitigation: extend the redaction tests when adding sensitive attributes.

## Operational and review guidance

- Revisit this document when adding payment flows, new roles, public endpoints, or cross-organization features.
- Run `go test ./...`, `go test -race ./...`, and CI security jobs before release.
- Track remaining compliance-oriented work (for example, retention jobs and OpenAPI publication) in `backlog.v1.json` under CAT-H and CAT-I.
