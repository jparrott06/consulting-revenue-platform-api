# Architecture Boundaries

This document defines where new code should live as V2 refactors continue.

## Layer responsibilities

- `internal/httpapi` (transport edge)
  - Decode/validate request DTOs and path params.
  - Enforce authn/authz guards.
  - Call use-case services.
  - Map typed use-case errors to API error envelope.
- `internal/usecase` (application orchestration)
  - Execute workflow steps using domain rules and repository interfaces.
  - Own transaction boundaries for multi-step operations.
  - Return typed errors/messages intended for transport mapping.
- `internal/domain` (business model)
  - Define statuses, invariants, and transition rules.
  - No HTTP, SQL, or framework dependencies.
- `internal/repo` (persistence)
  - Query and mutate storage.
  - No HTTP request parsing or role checks.
  - Implement interfaces consumed by use-case services.

## Dependency direction

- `httpapi` -> `usecase` -> (`domain`, `repo interfaces`)
- `repo` implements interfaces for `usecase`
- `domain` is dependency-minimal and imported by upper layers

Avoid importing `httpapi` or `repo` into `domain`.

## Validation, authz, and transactions

- **Validation**
  - Transport validation for request shape/type.
  - Domain validation for business invariants.
- **Authorization**
  - Role/membership checks at transport edge before use-case execution.
- **Transactions**
  - Start/commit/rollback are implemented at the repository boundary using `internal/db.RunInTx` for critical workflows; use-case methods call a single repository operation per HTTP action. See [transaction-matrix.md](transaction-matrix.md).

## Contribution checklist (API/workflow changes)

1. Define or update domain invariant first.
2. Add/update use-case method with typed errors.
3. Adapt repository implementation behind interface.
4. Keep handler changes focused on DTO parsing + error mapping.
5. Add tests at domain/use-case layer plus endpoint regression checks.
