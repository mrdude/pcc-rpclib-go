package apirt

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	pcommon "github.com/mrdude/pcc-common"
	"github.com/mrdude/pcc-rpclib-go/apistats"
	"github.com/mrdude/pcc-rpclib-go/apitrace"
	"github.com/mrdude/pcc-rpclib-go/apivalidation"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type CallHandlerFn func(ctx context.Context, w ResponseWriter, req *Request) (any, error)

type GeneratedCallHandler struct {
	// create this Operation's parameter object.
	// if nil, Empty will be used
	CreateParam func() any

	// create this Operation's return object.
	// if nil, Empty will be used
	CreateReturn func() any

	// executes this RPC
	ExecuteRPC OperationHook
}

var _ http.Handler = &ServerEndpoint{}

type ServerEndpoint struct {
	prefix string

	calls map[CallKey]GeneratedCallHandler

	DisableInputValidation bool // if true, RPC inputs will not be validated
}

func NewServerEndpoint(prefix string) *ServerEndpoint {
	return &ServerEndpoint{
		prefix: prefix,

		calls: make(map[CallKey]GeneratedCallHandler),
	}
}

func (ep *ServerEndpoint) Prefix() string {
	return ep.prefix
}

func (ep *ServerEndpoint) BindCall(key CallKey, impl GeneratedCallHandler) {
	ep.calls[key] = impl
}

func (ep *ServerEndpoint) ServeHTTP(httpWriter http.ResponseWriter, httpReq *http.Request) {
	// set up stats
	stats := apistats.NewServerCall()
	defer stats.Submit()

	// determine the serializer to use from the request's Content-Type
	var ser *Serializer
	var serializerFound bool
	if ct := httpReq.Header.Get("Content-Type"); ct == "" {
		ser, serializerFound = GetDefaultSerializer(), true
	} else {
		ct, _, _ = strings.Cut(ct, ";")
		ser = FindSerializerByMime(ct, nil)
		serializerFound = ser != nil

		// if we didn't find a suitable serializer, set "ser"
		// to a non-nil value
		if !serializerFound {
			ser = GetDefaultSerializer()
		}
	}

	if span := apitrace.GetSpanContext(httpReq.Context()); span != nil {
		if serializerFound {
			span.Tags["Serializer-Content-Type"] = ser.MimeType()
		} else {
			span.Tags["Serializer-Content-Type"] = "n/a"
		}
	}

	// apply client deadline, if available
	if v := httpReq.Header.Get(rpcDeadlineHeader); v != "" {
		ns, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			dl := time.UnixMicro(ns)

			var cancel context.CancelFunc
			ctx := httpReq.Context()
			ctx, cancel = context.WithDeadlineCause(ctx, dl, errors.New("client deadline exceeded"))
			defer cancel()
			httpReq = httpReq.WithContext(ctx)
		}
	}

	// wrap types
	w := wrapResponseWriter(httpWriter, ser, apitrace.GetSpanContext(httpReq.Context()), stats)
	req := wrapRequest(httpReq, 5*mb)
	defer req.Close()
	ctx := httpReq.Context()

	// check path prefix
	if !strings.HasPrefix(httpReq.URL.Path, ep.prefix) {
		w.WriteError(ctx, &Exception{
			err:     nil,
			status:  http.StatusBadRequest,
			message: "unknown call",
		})
		return
	}

	// check method
	if strings.ToUpper(httpReq.Method) != http.MethodPost {
		w.WriteError(ctx, &Exception{
			err:     nil,
			status:  http.StatusMethodNotAllowed,
			message: "unsupported method",
		})
		return
	}

	// check serializer
	if !serializerFound {
		w.WriteError(ctx, &Exception{
			err:     nil,
			status:  http.StatusUnsupportedMediaType,
			message: "unsupported Content-Type",
		})
		return
	}

	ctx = WithSerializer(ctx, ser)

	// calculate call key
	key, err := parseCallKey(httpReq.URL.Path, ep.prefix)
	if err != nil {
		w.WriteError(ctx, err)
		return
	}
	ctx = WithCallKey(ctx, key)
	stats.ApiName = key.ApiName
	stats.OperationName = key.OperationName
	ctx = pcommon.WithLoggerContextValue(ctx, pcommon.GetLogger(ctx).With(zap.String("prpc.call", key.ApiName+"."+key.OperationName)))

	if span := apitrace.GetSpanContext(httpReq.Context()); span != nil {
		span.Tags["ServiceName"] = key.ApiName
		span.Tags["OperationName"] = key.OperationName
	}

	// find implementation
	impl, ok := ep.calls[key]
	if !ok {
		w.WriteError(ctx, &Exception{
			err:     nil,
			status:  http.StatusBadRequest,
			message: "unknown call",
		})
		return
	}

	// generate OperationInput and OperationOutput objects
	opIn := &OperationInput{
		Header: req.Header(),
		Body:   &Empty{},
	}

	opOut := &OperationOutput{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       &Empty{},
	}

	if impl.CreateParam != nil {
		opIn.Body = impl.CreateParam()
	}

	if impl.CreateReturn != nil {
		opOut.Body = impl.CreateReturn()
	}

	// require HTTP/2 for BiDi streaming
	// TODO set ResponseController.EnableFullDuplex() for HTTP/1.1?
	isBidi := reflect.TypeOf(opIn.Body).Kind() == reflect.Chan && reflect.TypeOf(opOut.Body).Kind() == reflect.Chan
	if isBidi && httpReq.ProtoMajor < 2 {
		// Without this check, a bidirectional streaming call over HTTP/1.1 will
		// just block forever (or until the request times out)
		w.WriteError(ctx, NewException(http.StatusBadRequest, "Bidi streaming calls must be HTTP/2"))
		return
	}

	// create the errgroup
	g, gctx := errgroup.WithContext(ctx)
	gctx = withErrGroup(gctx, g)

	// execute RPC and write response
	err = invokeOperationWithHooks(gctx, w, req, g, nil, impl.ExecuteRPC)
	if err != nil && !isIgnorableError(err) {
		pcommon.GetLogger(ctx).Error("hooks failed", zap.String("err.type", reflect.TypeOf(err).String()), zap.Error(err))
	}

	err = g.Wait()
	if err != nil && !isIgnorableError(err) {
		pcommon.GetLogger(ctx).Error("prpc errgroup failed", zap.Error(err))
	}

	if headersWritten := w.(*responseWriterImpl).headersWritten.Load(); !headersWritten {
		pcommon.GetLogger(ctx).Error("handler did not write a response")
	}
}

// returns true if this is an error that doesn't need to be printed to screen
// is used by ServeHTTP() by filter errors
func isIgnorableError(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (ep *ServerEndpoint) validateInput(input any) error {
	if ep.DisableInputValidation {
		return nil
	}

	return apivalidation.ValidateObject(input)
}

func parseCallKey(path string, apiPrefix string) (key CallKey, err error) {
	path = path[len(apiPrefix)+1:]
	sp := strings.SplitN(path, "/", 2)

	if len(sp) != 2 {
		err = &Exception{
			err:     nil,
			status:  http.StatusBadRequest,
			message: "unknown call",
		}
		return
	}

	key.ApiName = sp[0]
	key.OperationName = sp[1]
	return
}
