package apirt

import (
	"context"
	"encoding/binary"
	"net/http"
	"sync/atomic"

	"github.com/mrdude/pcc-common"
	"github.com/mrdude/pcc-rpclib-go/apistats"
	"github.com/mrdude/pcc-rpclib-go/apitrace/zipkin"
	"go.uber.org/zap"
)

type ResponseWriter interface {
	Header() http.Header
	WriteOutput(ctx context.Context, body any)
	WriteMessage(ctx context.Context, body any) error
	WriteEofMessage(ctx context.Context) error
	WriteError(ctx context.Context, err error)
}

type responseWriterImpl struct {
	w     http.ResponseWriter
	ser   *Serializer
	span  *zipkin.Span // can be nil
	stats *apistats.Call

	headersWritten atomic.Bool
}

func wrapResponseWriter(w http.ResponseWriter, ser *Serializer, span *zipkin.Span, stats *apistats.Call) ResponseWriter {
	return &responseWriterImpl{
		w:     w,
		ser:   ser,
		span:  span,
		stats: stats,
	}
}

const streamingHeader = "X-API-Streaming"

func (w *responseWriterImpl) Header() http.Header {
	return w.Header()
}

// regular response
func (w *responseWriterImpl) WriteOutput(ctx context.Context, body any) {
	respBytes, err := w.ser.Marshal(body)
	if err != nil {
		w.doWriteError(ctx, err)

		if w.span != nil {
			w.span.AddAnnotation(zipkin.ServerSendAnnotation)
		}

		w.stats.StatusCode = CoerceErrorToException(err).EffectiveStatus()
	} else {
		w.doWriteUnaryResponse(ctx, respBytes)

		if w.span != nil {
			w.span.AddAnnotation(zipkin.ServerSendAnnotation)
		}

		w.stats.StatusCode = http.StatusOK
	}

	w.headersWritten.Store(true)
}

// streaming response
func (w *responseWriterImpl) WriteMessage(ctx context.Context, body any) (err error) {
	// streaming response
	if !w.headersWritten.Load() {
		w.w.Header().Set(streamingHeader, "resp")
		w.w.Header().Set("Transfer-Encoding", "chunked")
		w.w.Header().Set("Content-Type", w.ser.MimeType())

		w.w.WriteHeader(http.StatusOK)
		w.stats.StatusCode = http.StatusOK

		w.headersWritten.Store(true)
	}

	// write the message
	msgBytes, err := w.ser.Marshal(body)
	if err != nil {
		panic(err) // TODO handle this better
	}

	_, err = w.writeMessage(message{Payload: msgBytes})
	if err != nil {
		pcommon.GetLogger(ctx).Info("writing message failed", zap.Error(err))
		return
	}

	w.flush()
	if w.span != nil {
		w.span.AddAnnotation(zipkin.ServerSendAnnotation)
	}

	return
}

func (w *responseWriterImpl) WriteEofMessage(ctx context.Context) error {
	// streaming response
	if !w.headersWritten.Load() {
		w.w.Header().Set(streamingHeader, "resp")
		w.w.Header().Set("Transfer-Encoding", "chunked")
		w.w.Header().Set("Content-Type", w.ser.MimeType())

		w.w.WriteHeader(http.StatusOK)
		w.stats.StatusCode = http.StatusOK

		w.headersWritten.Store(true)
	}

	_, err := w.writeMessage(message{Type: eofMessage})
	return err
}

func (w *responseWriterImpl) WriteError(ctx context.Context, err error) {
	if w.headersWritten.Load() {
		return
	}

	w.doWriteError(ctx, err)

	if w.span != nil {
		w.span.AddAnnotation(zipkin.ServerSendAnnotation)
	}

	w.stats.StatusCode = CoerceErrorToException(err).EffectiveStatus()

	w.headersWritten.Store(true)
}

func (w *responseWriterImpl) writeMessage(msg message) (int, error) {
	msgBytes := msg.Encode()

	var buf []byte
	buf = binary.AppendUvarint(buf, uint64(len(msgBytes)))
	if len(msgBytes) > 0 {
		buf = append(buf, msgBytes...)
	}
	return w.w.Write(buf)
}

func (w *responseWriterImpl) flush() {
	f := w.w.(http.Flusher)
	f.Flush()
}

const errorMessageHeader = "X-API-Error"

func (w *responseWriterImpl) doWriteError(ctx context.Context, err error) {
	ex := CoerceErrorToException(err)

	pcommon.GetLogger(ctx).Error("Returning error",
		zap.NamedError("coercedErr", ex),
		zap.Error(err))

	w.w.Header().Set("Content-Type", w.ser.MimeType())
	w.w.Header().Set(errorMessageHeader, ex.EffectiveMessage())
	w.w.WriteHeader(ex.EffectiveStatus())
	w.w.Write(w.ser.GetEmptyBody())
}

func (w *responseWriterImpl) doWriteUnaryResponse(ctx context.Context, body []byte) {
	w.w.Header().Set("Content-Type", w.ser.MimeType())

	w.w.WriteHeader(http.StatusOK)
	w.w.Write(body)
}
