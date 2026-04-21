# Demo UI shell (N-03)

This is a lightweight frontend shell for showcasing key API workflows without adding a full product frontend.

## What it demonstrates

- Login (`POST /auth/login`)
- Tenant context (`GET /v1/me`)
- Core list views (`GET /v1/clients`, `GET /v1/projects`, `GET /v1/time-entries`)
- Workflow actions:
  - Submit/approve time entry
  - Generate invoice
  - Send invoice

Responses are rendered as structured JSON so reviewers can see exact API payloads and error envelopes.

## Run locally

Serve the `demo-ui` directory as static files:

```bash
cd demo-ui
python3 -m http.server 5173
```

Open [http://localhost:5173](http://localhost:5173).

Optional API override:

- Use query string: `http://localhost:5173/?baseUrl=http://localhost:8080`
- Or edit the base URL field in the UI.

## Token storage and security trade-off

- Access token is stored in `sessionStorage` for convenience between page refreshes.
- Trade-off: this is acceptable for local demo use, but not ideal for hardened production UX.
- Never use production credentials in this shell; use synthetic demo users only.

## CORS reminder

Set backend CORS origins to only your local demo origin(s), for example:

```bash
export CORS_ALLOWED_ORIGINS=http://localhost:5173
```

Do not use wildcard CORS in shared/staging environments.
