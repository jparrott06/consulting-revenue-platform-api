package httpapi

import (
	"go.opentelemetry.io/otel"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

func init() {
	otel.SetTracerProvider(nooptrace.NewTracerProvider())
}
