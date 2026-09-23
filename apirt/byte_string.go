package apirt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

var _ json.Marshaler = &ByteString{}
var _ json.Unmarshaler = &ByteString{}

// ByteString represents a []byte that should not be modified
type ByteString struct {
	// this is public purely so that blockIO can reference it
	// without including apirt

	Buf []byte
}

func NewByteStringFromBytes(b []byte) ByteString {
	return ByteString{Buf: b}
}

func (b ByteString) Bytes() []byte {
	return b.Buf
}

func (b ByteString) Copy() ByteString {
	return ByteString{Buf: bytes.Clone(b.Buf)}
}

func (b ByteString) MarshalJSON() ([]byte, error) {
	str := base64.URLEncoding.EncodeToString(b.Buf)
	return json.Marshal(str)
}

func (b *ByteString) UnmarshalJSON(js []byte) (err error) {
	var str string
	err = json.Unmarshal(js, &str)
	if err != nil {
		return
	}

	b.Buf, err = base64.URLEncoding.DecodeString(str)
	return err
}
