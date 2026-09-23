// Package apitrace contains helper functions related to Zipkin tracing in prpc
package apitrace

import (
	"context"
	"time"

	"github.com/mrdude/pcc-rpclib-go/apitrace/zipkin"
)

var defaultForwarder Forwarder = NewNilForwarder()

func SetProcessDefaultForwarder(f Forwarder) {
	if f == nil {
		defaultForwarder = NewNilForwarder()
	} else {
		defaultForwarder = f
	}
}

func GetProcessDefaultForwarder() Forwarder {
	return defaultForwarder
}

func NewSpan(kind zipkin.Kind, parent *zipkin.SpanHeaders) *zipkin.Span {
	s := &zipkin.Span{
		Ts:   zipkin.Time{Time: time.Now()},
		Kind: kind,
		Tags: make(map[string]string),
	}

	if parent != nil {
		oldSpanId := parent.SpanId

		s.TraceId = parent.TraceId
		s.SpanId = zipkin.RandomSpanId()
		s.ParentSpanId = &oldSpanId
	} else {
		s.TraceId = zipkin.RandomTraceId()
		s.SpanId = zipkin.RandomSpanId()
		s.ParentSpanId = nil
	}

	return s
}

func NewChildSpanFromContext(ctx context.Context, kind zipkin.Kind) *zipkin.Span {
	s := GetSpanContext(ctx)
	if s == nil {
		return NewSpan(kind, nil)
	}

	return NewSpan(kind, &s.SpanHeaders)
}
