# API Compatibility Policy

This policy defines how API changes are classified and shipped so consumers can upgrade safely.

## Compatibility classes

- **Additive (backward compatible)**
  - Adds new endpoints, optional request fields, optional response fields, or new enum values where clients are expected to ignore unknown values.
  - Does not remove or rename existing fields/paths.
- **Behavior-changing (review required)**
  - Keeps schema shape but changes workflow semantics, validation strictness, ordering, defaults, or side effects.
  - Must include migration notes and explicit test coverage for old vs new behavior.
- **Breaking (not backward compatible)**
  - Removes/renames endpoints, methods, fields, or enum members.
  - Changes requiredness, data types, or error code contracts in a way that can break existing clients.
  - Requires explicit approval and a deprecation plan before merge.

## Deprecation policy

1. Mark the route/field as deprecated in `docs/openapi.yaml` with guidance.
2. Keep deprecated behavior for at least one release cycle unless there is a security emergency.
3. Add migration notes in PR description and docs (README/runbook or endpoint-specific docs).
4. Remove only after replacement is available and consumers have notice.

## Versioning rules

- Current API uses a stable `v1` route namespace.
- Additive changes can ship in `v1` with updated OpenAPI and tests.
- Breaking changes should be avoided; if unavoidable, they require explicit classification and staged rollout notes.
- Emergency security changes may shorten deprecation timelines, but must be documented as exceptions in the PR.

## Required checks for API-touching PRs

Any PR that changes HTTP routes, request/response payloads, or error envelopes must:

1. Update `docs/openapi.yaml` in the same PR.
2. Classify the change as additive, behavior-changing, or breaking.
3. Keep OpenAPI/runtime parity checks green (`TestOpenAPIRouteCoverage`), and update internal-route allowlist only for intentionally undocumented endpoints.
4. Include tests for changed behavior (or explain why existing coverage is sufficient).
5. Update relevant docs (README/runbook) when consumer behavior changes.

## Example classifications

- **Additive**
  - Add `GET /v1/admin/reconciliation-summary` endpoint.
  - Add optional `aligned` field in a response.
- **Behavior-changing**
  - Same endpoint shape, but stricter validation now rejects previously accepted inputs.
  - Same status code, but transition semantics change in a workflow.
- **Breaking**
  - Rename `invoice_id` to `id` in a required response field.
  - Change an endpoint from `POST` to `PUT` or remove a `v1` route.
  - Remove an enum value clients may still send.
