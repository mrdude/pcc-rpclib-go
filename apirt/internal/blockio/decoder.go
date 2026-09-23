package blockio

import (
	"encoding/binary"
	"io"
)

type decoder struct {
	Input []byte
	Pos   int
}

func (dec *decoder) Eof() bool {
	return dec.Pos >= len(dec.Input)
}

func (dec *decoder) Read(p []byte) (n int, err error) {
	bytesLeft := len(dec.Input) - dec.Pos
	if dec.Pos == len(dec.Input) {
		n = 0
		err = io.EOF
		return
	} else {
		if len(p) <= bytesLeft {
			n = len(p)
		} else {
			n = bytesLeft
		}

		err = nil

		copy(p, dec.Input[dec.Pos:dec.Pos+n])

		dec.Pos += n
		return
	}
}

func (dec *decoder) ReadByte() (byte, error) {
	if dec.Eof() {
		return 0, io.EOF
	}

	p := dec.Pos
	dec.Pos++
	return dec.Input[p], nil
}

func (dec *decoder) ReadFieldType() (fieldType, error) {
	b, err := dec.ReadByte()
	if err != nil {
		return 0, err
	}

	return fieldType(b), nil
}

func (dec *decoder) ReadByteString() ([]byte, error) {
	byteLen, err := binary.ReadUvarint(dec)
	if err != nil {
		return nil, err
	}

	b := make([]byte, byteLen)
	_, err = io.ReadFull(dec, b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (dec *decoder) ReadString() (string, error) {
	b, err := dec.ReadByteString()
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (dec *decoder) ReadInt() (val int64, err error) {
	val, err = binary.ReadVarint(dec)
	return
}

func (dec *decoder) ReadUint() (val uint64, err error) {
	val, err = binary.ReadUvarint(dec)
	return
}
