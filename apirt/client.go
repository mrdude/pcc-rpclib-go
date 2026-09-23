package apirt

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mrdude/pcc-common"
	"github.com/mrdude/pcc-rpclib-go/apistats"
	"github.com/mrdude/pcc-rpclib-go/apitrace"
	"github.com/mrdude/pcc-rpclib-go/apitrace/zipkin"
	"github.com/mrdude/pcc-rpclib-go/httpx"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	rpcDeadlineHeader     = "X-API-Deadline-Micros"
	streamingRpcKeepAlive = 5 * time.Second
	streamingRpcTimeout   = 2 * streamingRpcKeepAlive
)

const (
	kb = 1024
	mb = 1024 * kb
	gb = 1024 * mb
)

type ClientRequest struct {
	Call   CallKey
	Header http.Header
	Param  any // either an object or a chan *object
}

type ClientResponse struct {
	StatusCode int
	Header     http.Header
	Body       any // either an object or a chan *object
}

type Client struct {
	urlPrefix string

	httpCl *httpx.Client

	Serializer *Serializer        // the serializer that is used for all requests. If nil, we default to JSON serialization
	Forwarder  apitrace.Forwarder // the forwarder that is used for all requests. If nil, we default to apitrace.GetProcessDefaultForwarder
}

func NewClient(cl *httpx.Client, urlPrefix string) *Client {
	return &Client{
		urlPrefix: urlPrefix,

		httpCl: cl,
	}
}

func (cl *Client) buildRequestUrl(req *ClientRequest) (urlStr string) {
	urlStr = cl.urlPrefix

	if urlStr != "" {
		urlStr = strings.TrimSuffix(urlStr, "/")
		urlStr = strings.TrimPrefix(urlStr, "/")
		urlStr = "/" + urlStr
	}

	urlStr += fmt.Sprintf("/%s/%s", req.Call.ApiName, req.Call.OperationName)
	return urlStr
}

func (cl *Client) serializer() *Serializer {
	if ser := cl.Serializer; ser == nil {
		return GetDefaultSerializer()
	} else {
		return ser
	}
}

func (cl *Client) forwarder() apitrace.Forwarder {
	if f := cl.Forwarder; f == nil {
		return apitrace.GetProcessDefaultForwarder()
	} else {
		return f
	}
}

func (cl *Client) Do(ctx context.Context, g *errgroup.Group, req *ClientRequest, resp *ClientResponse) (err error) {
	// set up stats
	stats := apistats.NewClientCall()
	defer stats.Submit()

	stats.ApiName = req.Call.ApiName
	stats.OperationName = req.Call.OperationName

	// create a span
	span := apitrace.NewChildSpanFromContext(ctx, zipkin.ClientKind)
	span.Shared = true
	ctx = apitrace.WithSpanContext(ctx, span)

	defer cl.forwarder().SubmitSpan(*span)

	// choose a serializer
	ser := cl.serializer()

	// convert the ClientRequest to an *http.Request
	if req.Param == nil {
		req.Param = &Empty{}
	}

	var reqBody io.Reader
	if reflect.TypeOf(req.Param).Kind() == reflect.Chan {
		// streaming request body
		r, w := io.Pipe()
		reqBody = r

		g.Go(func() (err error) {
			var (
				closed bool // have we hit EOF on the req.Param channel?
			)

			writeMessage := func(m message) error {
				msg := m.Encode()

				var buf []byte
				buf = binary.AppendUvarint(buf, uint64(len(msg)))
				if len(msg) > 0 {
					buf = append(buf, msg...)
				}
				_, err := w.Write(buf)
				return err
			}

			defer func() {
				// if we haven't hit EOF on the input channel,
				// drain the channel
				if !closed {
					for {
						_, ok := reflect.ValueOf(req.Param).Recv()
						if !ok {
							break
						}
					}
				}
			}()

			defer func() {
				if err == nil || errors.Is(err, io.EOF) {
					// write EOF message
					writeMessage(message{Type: eofMessage})
					w.Close()
					span.AddAnnotation(zipkin.ClientSendAnnotation)
				} else {
					w.CloseWithError(err)
				}
			}()

			// TODO convert this to a startWatchdogReader function?
			// start the watchdog ticker
			watchdog := time.NewTicker(streamingRpcKeepAlive)
			defer watchdog.Stop()

			for {
				chosen, v, ok := reflect.Select([]reflect.SelectCase{
					reflect.SelectCase{
						Dir:  reflect.SelectRecv,
						Chan: reflect.ValueOf(req.Param),
					},
					reflect.SelectCase{
						Dir:  reflect.SelectRecv,
						Chan: reflect.ValueOf(watchdog.C),
					},
				})
				switch chosen {
				case 0:
					// handle the received value
					//v, ok := reflect.ValueOf(req.Param).Recv()
					if !ok {

						err = io.EOF
						closed = true
						return
					}

					var b []byte
					b, err = ser.Marshal(v.Interface())
					if err != nil {
						pcommon.GetLogger(ctx).Warn("failed to marshal input streaming object",
							zap.Error(err))
						return
					}

					err = writeMessage(message{Payload: b})
					if err != nil {
						return
					}
					span.AddAnnotation(zipkin.ClientSendAnnotation)

					// reset watchdog timer
					watchdog.Reset(streamingRpcKeepAlive)
				case 1:
					// send a keep-alive message

					err = writeMessage(message{Type: keepAliveMessage})
					if err != nil {
						return
					}
				}
			}
		})
	} else {
		var reqBodyBytes []byte
		reqBodyBytes, err = ser.Marshal(req.Param)
		if err != nil {
			err = fmt.Errorf("failed to marshal client body: %w", err)
			return
		}

		reqBody = bytes.NewReader(reqBodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", cl.buildRequestUrl(req), reqBody)
	if err != nil {
		err = fmt.Errorf("failed to create request: %w", err)
		return
	}

	for k, vv := range req.Header {
		httpReq.Header[textproto.CanonicalMIMEHeaderKey(k)] = vv
	}

	if reflect.TypeOf(req.Param).Kind() == reflect.Chan {
		httpReq.Header.Set("Transfer-Encoding", "chunked")
	}

	httpReq.Header.Set("Content-Type", ser.MimeType())

	if t, ok := ctx.Deadline(); ok {
		httpReq.Header.Set(rpcDeadlineHeader, strconv.FormatInt(t.UnixMicro(), 10))
	}

	// include tracing headers on the outgoing request
	span.SetHeaders(httpReq.Header)

	// send the request
	if isStreamingRequest := reflect.TypeOf(req.Param).Kind() == reflect.Chan; !isStreamingRequest {
		span.AddAnnotation(zipkin.ClientSendAnnotation)
	}
	var httpResp *http.Response
	httpResp, err = cl.httpCl.Do(httpReq)
	if err != nil {
		return
	}
	// We call "defer httpResp.Body.Close()" farther down, so there is no need to put it here

	stats.StatusCode = httpResp.StatusCode

	// handle error responses
	isSuccess := httpResp.StatusCode >= 200 && httpResp.StatusCode <= 299
	if !isSuccess {
		defer httpResp.Body.Close()

		// decode the exception
		ex := &Exception{
			message: httpResp.Status,
			status:  httpResp.StatusCode,
		}

		if errorMessage := httpResp.Header.Get(errorMessageHeader); errorMessage != "" {
			ex.message = errorMessage
		}

		// return the exception
		err = ex
		return
	}

	// is this a streaming response?
	isStreamingResponse := httpResp.Header.Get(streamingHeader) == "resp"

	// read the non-streaming response body
	if !isStreamingResponse {
		defer httpResp.Body.Close()

		// read the response body
		var respBody []byte
		respBody, err = io.ReadAll(httpResp.Body)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}
		span.AddAnnotation(zipkin.ClientRecvAnnotation)

		// deserialize the response
		err = ser.Unmarshal(respBody, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal body (%s): %w", string(respBody), err)
		}

		resp.StatusCode = httpResp.StatusCode
		resp.Header = httpResp.Header

		return
	}

	// handle the streaming response body
	respCh := reflect.ValueOf(resp.Body) // this should be a chan object
	if respCh.Kind() != reflect.Chan {
		defer httpResp.Body.Close()

		err = errors.New("internal error: expected a channel")
		return
	}

	pointerToChannelType := respCh.Type().Elem()
	if pointerToChannelType.Kind() != reflect.Pointer {
		defer httpResp.Body.Close()

		err = errors.New("internal error: expected a channel of pointers")
		return
	}

	g.Go(func() (err error) {
		// this goroutine reads message's from the HTTP body, decodes them
		// to objects, and sends them to respCh.

		logger := pcommon.GetLogger(ctx)

		defer httpResp.Body.Close()
		defer respCh.Close()
		defer logger.Info("shutting down response streaming goroutine")

		byteReader := &byteReadWrapper{r: httpResp.Body} // TODO use a bufio.Reader?

		for {
			// read the message
			var v uint64
			v, err = binary.ReadUvarint(byteReader)
			if err != nil {
				// TODO how to handle this error?
				if errors.Is(err, io.EOF) {
					logger.Error("EOF reached", zap.Error(err))
					//time.Sleep(time.Second)
					//continue
					return
				} else {
					logger.Error("error while reading message header from body", zap.Error(err))
					return
				}
			}

			if v > uint64(gb) {
				// failed sanity check
				// TODO how to handle this error?
				logger.Error("message body failed sanity check")
				return
			}

			msgBytes := make([]byte, v)
			_, err = io.ReadFull(httpResp.Body, msgBytes)
			if err != nil {
				// failed to read the entire message
				// TODO how to handle this error
				logger.Error("error while reading message from body", zap.Error(err))
				return
			}

			// decode the message
			msg := message{}
			if err = msg.Decode(msgBytes); err != nil {
				// failed to decode message
				// TODO how to handle this error
				logger.Error("error while decoding message from body", zap.Error(err))
				return
			}

			// process the message
			if msg.Type == dataMessage {
				// decode the message and send it on the channel
				obj := reflect.New(pointerToChannelType.Elem())
				err = ser.Unmarshal(msg.Payload, obj.Interface())
				if err != nil {
					return
				}

				span.AddAnnotation(zipkin.ClientRecvAnnotation)
				respCh.Send(obj)
			} else if msg.Type == eofMessage {
				break
			} else if msg.Type == keepAliveMessage {
				// Do nothing
			} else {
				panic(fmt.Errorf("expected data or eof message, got %d", msg.Type))
			}
		}

		return nil
	})

	return
}

type byteReadWrapper struct {
	r io.ReadCloser
}

func (r *byteReadWrapper) ReadByte() (byte, error) {
	var b [1]byte
	n, err := r.r.Read(b[:])
	if err != nil {
		return 0, err
	}

	if n != 1 {
		return 0, errors.New("could not read byte")
	}

	return b[0], nil
}
