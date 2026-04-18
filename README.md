# Consulting Revenue Platform API (Go) - V1 Backlog Repo

This repository currently contains a production-oriented V1 planning backlog for a multi-tenant Go API that covers:

- Identity, tenancy, and RBAC
- Time tracking and approvals
- Invoicing and PDF generation
- Stripe-backed payment reconciliation
- Immutable ledger and audit logs
- Security/privacy controls and operational hardening

The goal is to provide enough low-level implementation detail that LLM coding agents can build the entire project from these stories with minimal ambiguity.

## Current Files

- `backlog.v1.json` - Agent-ready story backlog with dependencies, acceptance criteria, security/privacy requirements, edge cases, and test plans.
- `backlog.v1.schema.json` - Strict JSON Schema for validating `backlog.v1.json`.
- `backlog.v1.strict.json` - Strict backlog format with automation metadata per story.
- `agent.autonomy.rules.json` - Machine-readable autonomous workflow and merge policy.
- `story-status.json` - Story execution tracker for agents.
- `AGENTS.md` - Human-readable execution rules for autonomous coding agents.
- `.github/workflows/ci.yml` - PR/main checks for schema validation, lint, tests, race tests, and build.
- `.github/workflows/cd-main.yml` - Main-branch CD placeholder triggered after successful CI.
- `.github/pull_request_template.md` - Required PR sections for traceable autonomous changes.
- `branch-protection-checklist.md` - Step-by-step GitHub settings checklist for protected autonomous delivery.
- `scripts/agent-bootstrap.sh` - Preflight script for agents (validation + optional local setup).

## Intended V1 Product Scope

The target backend is a multi-tenant API where each tenant is an `organization`.

Primary domain flow:

1. Users authenticate and operate within an organization.
2. Contractors create and submit time entries.
3. Owner/accountant approve entries.
4. System generates invoices from approved work.
5. Stripe payment links are created for sent invoices.
6. Webhooks are processed idempotently.
7. Payment outcomes reconcile into invoice state and ledger events.

## Multi-Tenancy Model

The backlog assumes shared database + shared schema with strict row-level tenant filtering:

- Every business table includes `organization_id`.
- Every tenant-scoped query filters by `organization_id`.
- Authorization checks enforce role and membership before data access.
- Security test stories explicitly validate cross-tenant isolation and IDOR prevention.

## Security and Privacy Baseline

The backlog includes stories for:

- Token rotation and session revocation
- Signature verification for Stripe webhooks
- Idempotency for retry/replay safety
- Request validation and size limits
- Log redaction and data classification
- Optional encryption at rest for selected fields
- Threat modeling and retention controls

For a concise mapping of threats to controls and tests, see [docs/threat-model.md](docs/threat-model.md).

No secrets should ever be committed. Keep all sensitive values in environment variables or a secret manager.

## How to Use This Backlog

1. Open `backlog.v1.json`.
2. Execute by phase (`PHASE-1` through `PHASE-4`) or by dependency graph.
3. For each story:
   - implement code + migrations
   - satisfy acceptance criteria
   - add/update tests listed in `test_plan`
   - update docs/OpenAPI where relevant
4. Treat `global_constraints` as mandatory for every story.

Validation:

- `python -m jsonschema -i backlog.v1.json backlog.v1.schema.json`
- `./scripts/agent-bootstrap.sh`

Strict mode:

- Use `backlog.v1.strict.json` as the agent planning input.
- Update `story-status.json` on every story transition (`pending` -> `in_progress` -> `in_review` -> `done`).

## Suggested Agent Workflow

- Parallelize stories across categories when dependencies are complete.
- Keep one story per branch/PR to simplify review.
- Run tests and lint checks per story before merge.
- Prioritize all `P0` stories first.
- Follow the exact branch/PR/merge contract in `AGENTS.md` and `agent.autonomy.rules.json`.

## Autonomous PR and Merge Setup

To allow agents to open PRs and merge to `main` without your manual gate each time:

1. Link this folder to a GitHub repo and push default branch.
2. In GitHub repo settings:
   - enable **Allow auto-merge**
   - enable **Automatically delete head branches**
3. Configure branch protection on `main`:
   - require pull request before merging
   - require status checks to pass before merging
   - set required checks:
     - `schema-validate`
     - `lint`
     - `unit-test`
     - `race-test`
     - `build`
4. Ensure `gh` CLI is authenticated for the agent runtime account:
   - `gh auth status`
5. Agents can then use:
   - `gh pr create ...`
   - `gh pr merge --squash --delete-branch --auto`

This setup keeps autonomous delivery safe: agents can merge only when required CI checks are green.

## Next Steps

When you are ready to start implementation, create the initial scaffolding from:

- `A-01` through `A-05`
- `B-01` through `B-05`

These establish the foundation required for safe and correct development of all downstream features.
