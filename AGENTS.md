# Autonomous Agent Rules

This file defines how coding agents should operate in this repo with minimal user input.

## Source of Truth

- Backlog stories: `backlog.v1.json`
- Backlog schema: `backlog.v1.schema.json`
- Autonomous policy: `agent.autonomy.rules.json`

Agents should not invent scope outside backlog story fields unless required for security, correctness, or CI pass conditions.

## Working Contract

1. Choose one story whose dependencies are already complete.
2. Create a dedicated branch from `main`.
3. Implement only the story scope and mandatory cross-cutting constraints.
4. Add/adjust tests listed in the story `test_plan`.
5. Run local checks before opening a PR.
6. Open PR to `main` with required sections.
7. Merge only when all required checks are green.

## Branch and PR Rules

- Branch naming format: `feat/{story_id}-{short-slug}` (or `fix/`, `chore/`, `docs/`, `refactor/`, `test/` when appropriate)
- Include story ID in commit subject and PR title.
- One story per PR unless explicitly allowed by phase grouping.
- Use squash merge and delete the branch after merge.

## Required Local Checks

Run all available checks locally before PR:

- `go fmt ./...`
- `go test ./...`
- `go test -race ./...`

If available in repo tooling:

- `golangci-lint run`
- `go vet ./...`
- `gosec ./...`

If a command cannot run because project scaffolding is incomplete, note it explicitly in the PR under test evidence.

## Mandatory PR Sections

Each PR body must include:

- Summary
- Story ID
- Acceptance Criteria Mapping
- Security and Privacy Notes
- Test Evidence
- Risks and Follow-ups

## Merge Policy

Agents may merge autonomously when all required CI checks pass and branch protections are satisfied.

Preferred command:

`gh pr merge --squash --delete-branch --auto`

Never merge if required checks are failing or pending.

## Safety and Escalation

Agents must pause and request human input before:

- Destructive data migrations
- Backward-incompatible API changes
- Security trade-offs that reduce protections for delivery speed

Forbidden actions:

- Force pushing to `main`
- Committing secrets, tokens, keys, or credential files
- Bypassing required checks

## Secrets and Privacy

- Keep all secrets in environment variables or secret managers.
- Never log token values, auth headers, or webhook secrets.
- Use fake or synthetic data for tests and examples.
