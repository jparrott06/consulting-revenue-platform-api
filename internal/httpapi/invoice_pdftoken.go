package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

var (
	errPDFTokenMalformed = errors.New("malformed pdf token")
	errPDFTokenExpired   = errors.New("pdf token expired")
	errPDFTokenBadSig    = errors.New("invalid pdf token signature")
)

type pdfTokenPayload struct {
	Org       string `json:"org"`
	Invoice   string `json:"inv"`
	ExpiresAt int64  `json:"exp"`
}

func invoicePDFHMACKey(cfg config.Config) []byte {
	if s := strings.TrimSpace(cfg.InvoicePDFTokenSecret); s != "" {
		return []byte(s)
	}
	return []byte(cfg.JWTSigningKey)
}

// mintInvoicePDFToken returns an opaque token and its wall-clock expiry.
// ttl must be positive in production; callers enforce defaults.
func mintInvoicePDFToken(cfg config.Config, organizationID, invoiceID uuid.UUID, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	expiresAt = time.Now().UTC().Add(ttl)
	payload := pdfTokenPayload{
		Org:       organizationID.String(),
		Invoice:   invoiceID.String(),
		ExpiresAt: expiresAt.Unix(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, err
	}
	mac := hmac.New(sha256.New, invoicePDFHMACKey(cfg))
	_, _ = mac.Write(body)
	sig := mac.Sum(nil)
	token = base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig)
	return token, expiresAt, nil
}

func parseInvoicePDFToken(cfg config.Config, token string) (organizationID, invoiceID uuid.UUID, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return uuid.Nil, uuid.Nil, errPDFTokenMalformed
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return uuid.Nil, uuid.Nil, errPDFTokenMalformed
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, uuid.Nil, errPDFTokenMalformed
	}
	mac := hmac.New(sha256.New, invoicePDFHMACKey(cfg))
	_, _ = mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return uuid.Nil, uuid.Nil, errPDFTokenBadSig
	}
	var payload pdfTokenPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return uuid.Nil, uuid.Nil, errPDFTokenMalformed
	}
	if time.Now().UTC().Unix() > payload.ExpiresAt {
		return uuid.Nil, uuid.Nil, errPDFTokenExpired
	}
	organizationID, err = uuid.Parse(payload.Org)
	if err != nil {
		return uuid.Nil, uuid.Nil, errPDFTokenMalformed
	}
	invoiceID, err = uuid.Parse(payload.Invoice)
	if err != nil {
		return uuid.Nil, uuid.Nil, errPDFTokenMalformed
	}
	return organizationID, invoiceID, nil
}
