package apitrace

import (
	"context"

	"github.com/mrdude/pcc-rpclib-go/apitrace/zipkin"
)

type contextKey string

const spanContextKey contextKey = "SPAN"

func WithSpanContext(ctx context.Context, span *zipkin.Span) context.Context {
	return context.WithValue(ctx, spanContextKey, span)
}

func GetSpanContext(ctx context.Context) *zipkin.Span {
	s := ctx.Value(spanContextKey)
	if s == nil {
		return nil
	}

	return s.(*zipkin.Span)
}
