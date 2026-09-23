package apirt

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
)

// Request represents an incoming RPC request being handled by the server
type Request struct {
	req *http.Request

	body *bufio.Reader
}

// TODO use the maxBodyLength parameter
func wrapRequest(req *http.Request, maxBodyLen int64) *Request {
	return &Request{
		req:  req,
		body: bufio.NewReader(req.Body),
	}
}

func (req *Request) Close() error {
	return req.req.Body.Close()
}

func (req *Request) Header() http.Header {
	return req.req.Header
}

func (req *Request) UnmarshalBody(v any, ser *Serializer) (err error) {
	var bodyBytes []byte
	bodyBytes, err = io.ReadAll(req.body) // TODO limit the number of bytes read
	if err != nil {
		return
	}

	// unserialize body
	if ser == nil {
		return errors.New("nil serializer")
	}

	err = ser.Unmarshal(bodyBytes, v)
	if err != nil {
		return
	}

	return
}

// ReadMessage reads a single message and returns it.
// Keep-alive messages are returned as msg=nil, err=nil
func (req *Request) ReadMessage() (msg []byte, keepAlive bool, err error) {
	var m message
	err = req.unmarshalStreamingMessage(&m)
	if err != nil {
		return
	}

	switch m.Type {
	case dataMessage:
		msg = m.Payload
	case eofMessage:
		err = io.EOF
	case keepAliveMessage:
		keepAlive = true
	}

	return
}

// unmarshalStreamingMessage reads a single message and decodes it
// returns io.EOF if the end of stream message was sent
func (req *Request) unmarshalStreamingMessage(v *message) error {
	// read message length
	msgLen, err := binary.ReadUvarint(req.body)
	if err != nil {
		return err
	}

	// read message
	msg := make([]byte, msgLen)
	_, err = io.ReadFull(req.body, msg) // TODO limit the number of bytes read
	if err != nil {
		return err
	}

	// decode the message
	if err = v.Decode(msg); err != nil {
		return err
	}

	return nil
}
