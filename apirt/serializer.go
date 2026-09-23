package apirt

import (
	"context"
	"encoding/json"

	"github.com/mrdude/pcc-rpclib-go/apirt/internal/blockio"
)

const serializerContextKey = contextKey("serializer")

func WithSerializer(ctx context.Context, ser *Serializer) context.Context {
	return context.WithValue(ctx, serializerContextKey, ser)
}

func GetSerializer(ctx context.Context) *Serializer {
	ser := ctx.Value(serializerContextKey)
	if ser == nil {
		return nil
	}

	return ser.(*Serializer)
}

type Serializer struct {
	name string
	mime string

	marshalFn   func(v any) ([]byte, error)
	unmarshalFn func(data []byte, v any) error
	emptyBodyFn func() []byte
}

type decoderImpl interface {
	Decode(v any) error
}

func (ser *Serializer) Name() string {
	return ser.name
}

func (ser *Serializer) MimeType() string {
	return ser.mime
}

func (ser *Serializer) Marshal(v any) ([]byte, error) {
	return ser.marshalFn(v)
}

func (ser *Serializer) Unmarshal(data []byte, v any) error {
	return ser.unmarshalFn(data, v)
}

func (ser *Serializer) GetEmptyBody() []byte {
	return ser.emptyBodyFn()
}

var jsonSerializer = &Serializer{
	name:        "json",
	mime:        "application/json",
	marshalFn:   json.Marshal,
	unmarshalFn: json.Unmarshal,
	emptyBodyFn: func() []byte {
		return []byte("{}")
	},
}

var blockIOSerializer = &Serializer{
	name:        "block-io",
	mime:        "application/x.block-io",
	marshalFn:   blockio.Marshal,
	unmarshalFn: blockio.Unmarshal,
	emptyBodyFn: func() []byte { return []byte("") },
}

var serializers = []*Serializer{
	jsonSerializer,
	blockIOSerializer,
}

func GetDefaultSerializer() *Serializer {
	return serializers[0]
}

func GetJsonSerializer() *Serializer {
	return jsonSerializer
}

func FindSerializerByMime(mimeType string, defaultSerializer *Serializer) *Serializer {
	for _, ser := range serializers {
		if ser.mime == mimeType {
			return ser
		}
	}

	return defaultSerializer
}
