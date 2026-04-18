package webhookworker

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
	"github.com/stripe/stripe-go/v81"
)

type refundDTO struct {
	ID            string            `json:"id"`
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	Status        string            `json:"status"`
	PaymentIntent json.RawMessage   `json:"payment_intent"`
	Metadata      map[string]string `json:"metadata"`
}

type paymentIntentFailedDTO struct {
	ID               string `json:"id"`
	LastPaymentError *struct {
		Code string `json:"code"`
	} `json:"last_payment_error"`
}

func tryApplyStripeRefund(ctx context.Context, tx *sql.Tx, payload []byte) (handled bool, err error) {
	var ev stripe.Event
	if err := json.Unmarshal(payload, &ev); err != nil {
		return false, err
	}
	if ev.Type == stripe.EventTypeRefundFailed {
		return true, nil
	}
	if ev.Type != stripe.EventTypeRefundCreated && ev.Type != stripe.EventTypeRefundUpdated {
		return false, nil
	}

	var r refundDTO
	if err := json.Unmarshal(ev.Data.Raw, &r); err != nil {
		return true, err
	}
	if !strings.EqualFold(strings.TrimSpace(r.Status), string(stripe.RefundStatusSucceeded)) {
		return true, nil
	}
	pi := strings.TrimSpace(stripeJSONRefID(r.PaymentIntent))
	if pi == "" {
		return true, nil
	}
	in := repo.StripeRefundInput{
		StripeRefundID:        strings.TrimSpace(r.ID),
		StripePaymentIntentID: pi,
		AmountMinor:           r.Amount,
		Currency:              r.Currency,
	}
	return true, repo.ApplyStripeRefund(ctx, tx, in)
}

func tryRecordStripePaymentFailure(ctx context.Context, tx *sql.Tx, payload []byte) (handled bool, err error) {
	var ev stripe.Event
	if err := json.Unmarshal(payload, &ev); err != nil {
		return false, err
	}
	if ev.Type != stripe.EventTypePaymentIntentPaymentFailed {
		return false, nil
	}
	var pi paymentIntentFailedDTO
	if err := json.Unmarshal(ev.Data.Raw, &pi); err != nil {
		return true, err
	}
	code := ""
	if pi.LastPaymentError != nil {
		code = pi.LastPaymentError.Code
	}
	return true, repo.RecordStripePaymentFailure(ctx, tx, repo.StripePaymentFailureInput{
		StripePaymentIntentID: pi.ID,
		FailureCode:           code,
	})
}
