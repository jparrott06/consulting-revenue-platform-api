package stripepay

import (
	"fmt"
	"strings"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/paymentlink"
	"github.com/stripe/stripe-go/v81/price"
)

// CreateInvoicePaymentLink creates a Stripe Price and Payment Link for a one-time invoice amount.
// idempotencyKey is used as a prefix for Stripe idempotent requests (price and link).
func CreateInvoicePaymentLink(apiKey, idempotencyKey, currency string, unitAmountMinor int64, invoiceNumber int64, invoiceID string) (url, linkID string, err error) {
	stripe.Key = apiKey
	name := fmt.Sprintf("Invoice #%d", invoiceNumber)
	if len(name) > 500 {
		name = name[:500]
	}
	cur := strings.ToLower(strings.TrimSpace(currency))

	pr, err := price.New(&stripe.PriceParams{
		Params: stripe.Params{
			IdempotencyKey: stripe.String(idempotencyKey + ":price"),
		},
		Currency:   stripe.String(cur),
		UnitAmount: stripe.Int64(unitAmountMinor),
		ProductData: &stripe.PriceProductDataParams{
			Name: stripe.String(name),
		},
	})
	if err != nil {
		return "", "", err
	}

	plParams := &stripe.PaymentLinkParams{
		Params: stripe.Params{
			IdempotencyKey: stripe.String(idempotencyKey + ":link"),
		},
		LineItems: []*stripe.PaymentLinkLineItemParams{
			{
				Price:    stripe.String(pr.ID),
				Quantity: stripe.Int64(1),
			},
		},
	}
	plParams.AddMetadata("invoice_id", invoiceID)

	pl, err := paymentlink.New(plParams)
	if err != nil {
		return "", "", err
	}
	return pl.URL, pl.ID, nil
}
