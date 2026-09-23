package apirt

import (
	"errors"
	"fmt"
)

type messageType byte

const (
	dataMessage      messageType = 0
	eofMessage       messageType = 1
	keepAliveMessage messageType = 2
)

// message is a blob exchanged during a streaming RPC
type message struct {
	Type    messageType
	Payload []byte // the message payload, if any
}

func (m message) Encode() (buf []byte) {
	buf = append(buf, byte(m.Type))
	if len(m.Payload) > 0 {
		buf = append(buf, m.Payload...)
	}
	return
}

func (m *message) Decode(b []byte) error {
	if len(b) == 0 {
		return errors.New("invalid message")
	}

	// message type
	m.Type = messageType(b[0])

	// payload
	if len(b) > 1 {
		m.Payload = b[1:]
	}

	return nil
}

func (m message) String() string {
	return fmt.Sprintf("msg=%d bytes, type=[%d]%s", len(m.Payload), m.Type, m.Type.String())
}

func (t messageType) String() string {
	switch t {
	case dataMessage:
		return "data"
	case eofMessage:
		return "eof"
	case keepAliveMessage:
		return "keep-alive"
	default:
		return "unknown"
	}
}
