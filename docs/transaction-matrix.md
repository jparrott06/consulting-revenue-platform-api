# Transaction matrix (critical workflows)

This document maps money- and state-critical API flows to **where the database transaction starts and ends**. Handlers must not own transaction boundaries; they call use-case services, which delegate to repository functions that wrap `internal/db.RunInTx` or an equivalent single atomic unit.

| HTTP route | Use-case | Repository transaction boundary | Tables touched (within one tx) |
|------------|----------|----------------------------------|--------------------------------|
| `POST /v1/time-entries/{id}/submit` | `TimeEntryWorkflowService.Submit` | `repo.SubmitTimeEntry` → `applyTimeEntryTransition` | `time_entries`, `time_entry_events` |
| `POST /v1/time-entries/{id}/approve` | `TimeEntryWorkflowService.Approve` | `repo.ApproveTimeEntry` → `applyTimeEntryTransition` | `time_entries`, `time_entry_events` |
| `POST /v1/time-entries/{id}/reject` | `TimeEntryWorkflowService.Reject` | `repo.RejectTimeEntry` → `applyTimeEntryTransition` | `time_entries`, `time_entry_events` |
| `POST /v1/invoices/generate` | `InvoiceWorkflowService.Generate` | `repo.GenerateInvoiceFromApprovedEntries` | `invoice_sequences`, `invoices`, `invoice_line_items`, `time_entries` |
| `POST /v1/invoices/{id}/send` | `InvoiceWorkflowService.Send` | `repo.SendInvoice` | `invoices`, `ledger_entries` |

## Rules

- **One logical operation, one transaction** at the repository layer for these routes. Nested `BeginTx` calls are not used; `db.RunInTx` centralizes commit/rollback.
- **Rollback tests** that prove atomicity live in `internal/integrationtest/rollback_test.go` and require Postgres (`DATABASE_URL` in CI).
- **Logging**: failed transactions must not log raw request bodies or payment payloads (see `docs/logging.md`).

## Related code

- `internal/db/tx.go` — shared `RunInTx` helper.
- `docs/architecture-boundaries.md` — layering rules.
