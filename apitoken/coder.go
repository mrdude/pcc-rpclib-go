package apitoken

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	"github.com/mrdude/pcc-common"
	"github.com/mrdude/pcc-rpclib-go/apirt"
	"golang.org/x/crypto/nacl/secretbox"
)

// Coder is responsible for encoding and decoding tokens to JSON objects
type Coder interface {
	EncodeJsonToken(target any) *string
	DecodeJsonToken(encodedToken *string, target any) (isEmpty bool, err error)
}

type defaultCoder struct{}

func (*defaultCoder) EncodeJsonToken(target any) *string {
	if reflect.ValueOf(target).IsNil() {
		return nil
	}

	b, err := json.Marshal(target)
	if err != nil {
		panic(err)
	}

	return new(string(b))
}

func (*defaultCoder) DecodeJsonToken(encodedToken *string, target any) (isEmpty bool, err error) {
	if encodedToken == nil {
		isEmpty = true
	} else {
		isEmpty = false
		err = json.Unmarshal([]byte(*encodedToken), target)
		if err != nil {
			err = apirt.NewWrappedException(http.StatusBadRequest, "invalid token", err)
			return
		}
	}

	return
}

type encryptingCoder struct {
	keyName *[20]byte
	key     *[32]byte
}

func NewEncryptingCoder() Coder {
	var (
		keyName [20]byte
		key     [32]byte
	)
	copy(keyName[:], pcommon.GenerateCryptoRandomBytes(20))
	copy(key[:], pcommon.GenerateCryptoRandomBytes(32))
	return &encryptingCoder{
		keyName: &keyName,
		key:     &key,
	}
}

func (c *encryptingCoder) EncodeJsonToken(target any) *string {
	if reflect.ValueOf(target).IsNil() {
		return nil
	}

	// JSON encode
	b, err := json.Marshal(target)
	if err != nil {
		panic(err)
	}

	// encrypt
	var nonce [24]byte
	copy(nonce[:], pcommon.GenerateCryptoRandomBytes(24))

	var out []byte
	out = append(out, (*c.keyName)[:]...)
	out = append(out, nonce[:]...)
	out = secretbox.Seal(out, b, &nonce, c.key)

	// base64 encode
	encodedOut := base64.StdEncoding.EncodeToString(out)
	return &encodedOut
}

func (c *encryptingCoder) DecodeJsonToken(encodedToken *string, target any) (isEmpty bool, err error) {
	if encodedToken == nil {
		isEmpty = true
		return
	}

	// base64 decode
	var b []byte
	b, err = base64.StdEncoding.DecodeString(*encodedToken)
	if err != nil {
		err = apirt.NewWrappedException(http.StatusBadRequest, "invalid token", err)
		return
	}

	// decrypt
	var keyName [20]byte
	var nonce [24]byte
	copy(keyName[:], b[:20])
	b = b[20:]
	copy(nonce[:], b[:24])
	b = b[24:]

	if !bytes.Equal(keyName[:], (*c.keyName)[:]) {
		err = errors.New("mismatched keyname")
		err = apirt.NewWrappedException(http.StatusBadRequest, "invalid token", err)
		return
	}

	msg, ok := secretbox.Open(nil, b, &nonce, c.key)
	if !ok {
		err = errors.New("failed to decrypt message")
		err = apirt.NewWrappedException(http.StatusBadRequest, "invalid token", err)
		return
	}

	isEmpty = false
	err = json.Unmarshal(msg, target)
	if err != nil {
		err = apirt.NewWrappedException(http.StatusBadRequest, "invalid token", err)
		return
	}

	return
}
