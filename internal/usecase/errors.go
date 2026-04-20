package usecase

import "errors"

type ErrorKind string

const (
	ErrorKindUnauthorized ErrorKind = "unauthorized"
	ErrorKindForbidden    ErrorKind = "forbidden"
	ErrorKindValidation   ErrorKind = "validation"
	ErrorKindConflict     ErrorKind = "conflict"
	ErrorKindNotFound     ErrorKind = "not_found"
	ErrorKindInternal     ErrorKind = "internal"
)

type Error struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Kind)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func newError(kind ErrorKind, message string, cause error) error {
	return &Error{Kind: kind, Message: message, Cause: cause}
}

func Kind(err error) ErrorKind {
	var ue *Error
	if errors.As(err, &ue) {
		return ue.Kind
	}
	return ErrorKindInternal
}

func Message(err error) string {
	var ue *Error
	if errors.As(err, &ue) {
		return ue.Message
	}
	if err != nil {
		return err.Error()
	}
	return ""
}
