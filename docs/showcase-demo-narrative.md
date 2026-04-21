# Showcase demo narrative (N-01)

This narrative is the canonical portfolio walkthrough for the Consulting Revenue Platform API. It ties business value to concrete routes and objective success criteria.

## Personas

- **Owner (finance ops):** creates billable context, approves work, generates/sends invoices, reviews receivables.
- **Contractor:** logs and submits time for approval.
- **System:** enforces tenant/RBAC boundaries and records audit/ledger events.

All demo identities and emails must be synthetic.

## Canonical happy path

1. **Authenticate owner**
   - Route: `POST /auth/login`
   - Expected outcome: `200` with `access_token`, `refresh_token`, and `expires_in`.
   - Business value: owner can operate tenant-scoped workflows.

2. **Contractor submits time**
   - Route: `POST /v1/time-entries/{timeEntryID}/submit`
   - Expected outcome: `204`.
   - Business value: work moves from draft to reviewable state.

3. **Owner approves submitted time**
   - Route: `POST /v1/time-entries/{timeEntryID}/approve`
   - Expected outcome: `204`.
   - Business value: approved effort becomes invoice-eligible.

4. **Owner generates invoice from approved time**
   - Route: `POST /v1/invoices/generate`
   - Expected outcome: `201` with invoice payload including `id`, totals, status.
   - Business value: approved labor is transformed into receivable.

5. **Owner sends invoice**
   - Route: `POST /v1/invoices/{invoiceID}/send`
   - Expected outcome: `200` with invoice status `issued`.
   - Business value: receivable is now customer-facing and ready for payment collection.

6. **Owner verifies reporting output**
   - Route: `GET /v1/reports/outstanding`
   - Expected outcome: `200` with outstanding totals keyed by currency.
   - Business value: finance visibility confirms newly issued receivable is tracked.

## Objective success criteria

- **Authentication stage**
  - Access token is non-empty and accepted by tenant-protected endpoints.
- **Workflow transition stage**
  - Submit and approve endpoints return `204` with no workflow conflict.
- **Invoicing stage**
  - Generate returns exactly one invoice for the demo date range.
  - Send returns status `issued`.
- **Reporting stage**
  - Outstanding report contains the invoice currency with amount equal to generated total.
- **Auditability stage**
  - Demo script exits `0` and prints organization + resource identifiers for traceability.

## Expected role-mismatch failures (for demo credibility)

- Contractor calling owner-only invoice/report routes should return `403`.
- Owner attempting to approve a non-submitted entry should return `409`.
- Invalid UUID inputs should return `400` with standard API error envelope.

## Stripe-dependent fallback note

The canonical N-01 flow does not require live Stripe calls. If you include payment-link or webhook segments in a live demo, pre-configure `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET`; otherwise, explicitly state those steps are skipped in offline mode.
