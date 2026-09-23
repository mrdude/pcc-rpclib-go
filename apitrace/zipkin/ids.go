package zipkin

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

type TraceId string
type SpanId string

func RandomTraceId() TraceId {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return TraceId(strings.ToLower(hex.EncodeToString(b)))
}

func RandomSpanId() SpanId {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return SpanId(strings.ToLower(hex.EncodeToString(b)))
}
