package apitrace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/mrdude/pcc-common/server"
	"github.com/mrdude/pcc-common/server/serverctx"
	"github.com/mrdude/pcc-common/wide-event"
	"github.com/mrdude/pcc-rpclib-go/apitrace/zipkin"
)

const (
	tracingController contextKey = "tracing-controller"
	tracingAuthorized contextKey = "tracing-authorized"
)

type TracingController struct {
	Submit bool // if true, the trace will be submitted
	Span   *zipkin.Span
}

func WithTracingController(ctx context.Context, tc *TracingController) context.Context {
	return context.WithValue(ctx, tracingController, tc)
}

func GetTracingController(ctx context.Context) *TracingController {
	v := ctx.Value(tracingController)
	if v == nil {
		return nil
	}

	return v.(*TracingController)
}

func IsTracingAuthorized(ctx context.Context) bool {
	v := ctx.Value(tracingAuthorized)
	if v == nil {
		return false
	}

	b := v.(*bool)
	return *b
}

func WithAuthorizationHook(fn func(ctx context.Context) bool) server.OptionsFn {
	return server.WithGlobalMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			if fn(ctx) {
				ctx = context.WithValue(ctx, tracingAuthorized, new(true))
			}
			req = req.WithContext(ctx)

			next.ServeHTTP(w, req)
		})
	})
}

func CreateTracingMiddleware(fwd Forwarder, serviceName string) server.OptionsFn {
	if fwd == nil {
		fwd = GetProcessDefaultForwarder()
	}

	return server.WithGlobalMiddleware(
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := req.Context()

				// create tracing controller
				tc := TracingController{
					Submit: true,
					Span:   NewSpan(zipkin.ServerKind, nil),
				}
				ctx = WithTracingController(ctx, &tc)
				ctx = WithSpanContext(ctx, tc.Span)
				tc.Span.AddAnnotation(zipkin.ServerRecvAnnotation)

				// if this request is tracing authorized, reconfigure it as a shared server span
				//
				// In other words, we are trusting the client's "X-B3-*" headers
				if IsTracingAuthorized(ctx) {
					sp := zipkin.GetSpanHeaders(req.Header)
					if hasTraceHeaders := sp.TraceId != ""; hasTraceHeaders {
						tc.Span.TraceId = sp.TraceId
						tc.Span.SpanId = sp.SpanId
						tc.Span.ParentSpanId = sp.ParentSpanId
						tc.Span.Shared = true

						tc.Span.Sampled = sp.Sampled
						tc.Span.Debug = sp.Debug
					}
				}

				// wrap the response writer
				wrappedWriter := &responseWriterWrapper{delegate: w, span: tc.Span}

				// execute next handler and submit span
				newReq := req.WithContext(ctx)
				next.ServeHTTP(wrappedWriter, newReq)

				// submit span
				if tc.Submit {
					fwd.SubmitSpan(*tc.Span)
				}
			})
		},
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				span := GetSpanContext(req.Context())

				// attach the span information to the wide event
				if evt := wideevt.GetEvent(req.Context()); evt != nil {
					evt.TraceId = string(span.TraceId)
					evt.SpanId = string(span.SpanId)

					parentSpan := span.ParentSpanId
					if parentSpan != nil {
						evt.ParentSpanId = string(*parentSpan)
					}
				}

				// set span request variables
				span.Name = fmt.Sprintf("%s %s", req.Method, req.URL.Path)
				span.Tags["http.host"] = req.Host
				span.Tags["http.method"] = req.Method
				span.Tags["http.url"] = req.URL.Path

				// attach connection information
				conn := serverctx.GetConnection(req.Context())
				if addr := conn.LocalAddr(); addr != nil {
					span.LocalEndpoint = &zipkin.Endpoint{}

					if ip, err := netip.ParseAddrPort(addr.String()); err != nil {
						span.LocalEndpoint.SetAddrPort(ip)
					}

					if serviceName != "" {
						span.LocalEndpoint.SetService(serviceName)
					}
				}

				if addr := conn.RemoteAddr(); addr != nil {
					span.RemoteEndpoint = &zipkin.Endpoint{}

					if ip, err := netip.ParseAddrPort(addr.String()); err != nil {
						span.RemoteEndpoint.SetAddrPort(ip)
					}
				}

				// execute the next handler
				next.ServeHTTP(w, req)
			})
		},
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				span := GetSpanContext(req.Context())

				// sample 100% of traces
				// TODO in the future, do adaptive sampling, and "inherit" the Sampled and Debug headers of incoming authenticated requests
				span.Sampled = new(true)
			})
		},
	)
}

var _ http.ResponseWriter = (*responseWriterWrapper)(nil)
var _ http.Flusher = (*responseWriterWrapper)(nil)
var _ http.Hijacker = (*responseWriterWrapper)(nil)

type responseWriterWrapper struct {
	delegate http.ResponseWriter
	span     *zipkin.Span

	wroteHeader bool
}

func (w *responseWriterWrapper) doWriteHeader(statusCode int) {
	w.span.Tags["http.status"] = strconv.FormatInt(int64(statusCode), 10)

	if statusCode >= 200 && statusCode <= 299 {
		w.span.Tags["http.ok"] = "1"
	} else {
		w.span.Tags["http.ok"] = "0"
	}

	w.delegate.WriteHeader(statusCode)
}

func (w *responseWriterWrapper) Header() http.Header {
	return w.delegate.Header()
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.doWriteHeader(http.StatusOK)
	}

	n, err := w.delegate.Write(b)
	if err != nil {
		w.span.AddAnnotation(zipkin.ServerSendAnnotation)
	}
	return n, err
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.doWriteHeader(statusCode)
	}
}

func (w *responseWriterWrapper) Flush() {
	if f, ok := w.delegate.(http.Flusher); ok {
		if !w.wroteHeader {
			w.wroteHeader = true
			w.doWriteHeader(http.StatusOK)
		}

		f.Flush()
	}
}

func (w *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.delegate.(http.Hijacker); ok {
		return h.Hijack()
	}

	return nil, nil, errors.New("hijack is not supported")
}
