# End-to-end showcase guide (N-05)

Primary demo entrypoint for live walkthroughs and recorded portfolio sessions.

## 1) Pre-demo checklist (10-15 minutes)

- Checkout a known branch/commit (avoid drift during recording):
  - `git checkout main && git pull`
- Start dependencies:
  - PostgreSQL up and reachable via `DATABASE_URL`
- Configure local-only environment (synthetic creds only):
  - `APP_ENV=local`
  - `JWT_SIGNING_KEY=...`
  - Optional Stripe: `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`
- Apply schema and deterministic seed:
  - `make migrate-up`
  - `go run ./cmd/seed --reset --preset happy-path --contractors 1`

## 2) Readiness commands (copy/paste)

```bash
make openapi-validate
bash scripts/openapi-change-gate.sh
go test ./internal/httpapi -run TestOpenAPIRouteCoverage -count=1
go test ./internal/httpapi -run TestOpenAPIContract -count=1
go test ./...
go test -race ./...
go vet ./...
```

## 3) Live demo flow (CLI-first)

1. Start API: `make run`
2. Run scripted walkthrough with assertions:
   - `./scripts/demo-api.sh --report-file ./tmp/demo-api-report.json`
3. Optional isolation after run:
   - `./scripts/demo-api.sh --cleanup --report-file ./tmp/demo-api-report.json`

Expected output:

- Non-zero exit if any assertion fails.
- PASS summary with org + entity IDs.
- Machine-readable report at `./tmp/demo-api-report.json`.

## 4) UI-assisted walkthrough (optional)

1. Serve demo UI:
   - `cd demo-ui && python3 -m http.server 5173`
2. Open `http://localhost:5173`
3. Login and run key actions from buttons:
   - `GET /v1/me`, list routes, submit/approve, generate/send invoice.

Use this mode when you want response JSON visible while narrating workflow state transitions.

## 5) Recording checklist (security + evidence)

Before recording:

- Use only synthetic identities (`*@demo.local`).
- Hide shell history lines containing environment secrets.
- Confirm no bearer tokens or webhook secrets are visible on-screen.

Capture artifacts:

- `./tmp/demo-api-report.json` (script evidence of pass/fail checks)
- Key API logs (`request_id` visible, no secret values)
- `GET /metrics` snapshots for relevant workflow counters
- Screenshots:
  - demo UI logged-in state
  - generated invoice/send response
  - outstanding report payload

After recording:

- Remove transient artifacts that might include identifiers:
  - `rm -f ./tmp/demo-api-report.json`
- If sharing terminal output, redact org IDs and request IDs if policy requires.

## 6) Fallback paths

- **Stripe unavailable**: skip payment-link/webhook segments and state explicitly that Stripe steps are optional extension paths.
- **Slow local machine**: lower background load and run CLI demo only (skip UI) to reduce browser/recording overhead.
- **Seed reset blocked by immutable ledger rows**: use a fresh database or run without reset and include unique suffix in demo-script run.

## 7) Completion criteria for a successful showcase

- Script completes with PASS and generated report.
- Time entry transitions draft -> submitted -> approved -> invoiced are demonstrated.
- Invoice generation/send and outstanding report output are shown.
- Security hygiene (no secrets/tokens on screen) is preserved through final recording.
