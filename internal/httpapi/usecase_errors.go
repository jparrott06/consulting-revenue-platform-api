package httpapi

import (
	"context"
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/usecase"
)

func writeUsecaseError(ctx context.Context, w http.ResponseWriter, err error) {
	message := usecase.Message(err)
	switch usecase.Kind(err) {
	case usecase.ErrorKindUnauthorized:
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", message, nil)
	case usecase.ErrorKindForbidden:
		writeError(ctx, w, http.StatusForbidden, "forbidden", message, nil)
	case usecase.ErrorKindValidation:
		writeError(ctx, w, http.StatusBadRequest, "validation_error", message, nil)
	case usecase.ErrorKindConflict:
		writeError(ctx, w, http.StatusConflict, "conflict", message, nil)
	case usecase.ErrorKindNotFound:
		writeError(ctx, w, http.StatusNotFound, "not_found", message, nil)
	default:
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", message, nil)
	}
}
