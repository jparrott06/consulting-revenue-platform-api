package webhookworker

import (
	"context"
	"database/sql"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

// handleStripeWebhook applies business logic for a locked webhook row.
func handleStripeWebhook(ctx context.Context, tx *sql.Tx, row repo.StripeWebhookEventRow) error {
	in, disp, err := parseStripePaidReconcileInput(row.PayloadJSON)
	if err != nil {
		return err
	}
	switch disp {
	case stripePaidSkipSilent:
		return nil
	case stripePaidApply:
		invID, err := repo.ResolveStripePaidInvoiceID(ctx, tx, in.MetadataInvoiceID, in.StripePaymentLinkID)
		if err != nil {
			return err
		}
		in.InvoiceID = invID
		return repo.ReconcileStripePaymentPaid(ctx, tx, in)
	default:
		return nil
	}
}
