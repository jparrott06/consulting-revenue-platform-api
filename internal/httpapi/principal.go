package httpapi

import (
	"context"

	"github.com/google/uuid"
)

type principalCtxKey struct{}

// Principal carries authenticated tenant-scoped identity for a request.
type Principal struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Role           string
	SessionID      uuid.UUID
}

// WithPrincipal attaches a principal to the request context.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFromContext returns the principal if present.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	return p, ok
}
