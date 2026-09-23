package blockio

import (
	"encoding/binary"
	"errors"
	"reflect"
	"sync"
)

type encoder struct {
	Output []byte

	err error
}

func (enc *encoder) writeString(name string) {
	b := []byte(name)
	enc.Output = binary.AppendUvarint(enc.Output, uint64(len(b)))
	enc.Output = append(enc.Output, b...)
}

func (enc *encoder) writeType(typ fieldType) {
	enc.Output = append(enc.Output, byte(typ))
}

func (enc *encoder) Err() error {
	return enc.err
}

func (enc *encoder) EncodeValue(name string, val reflect.Value, typ reflect.Type) {
	if enc.err != nil {
		return
	}

	getEncodeFunc(typ)(enc, name, val)
}

func (enc *encoder) EncodeNil(name string) {
	if enc.err != nil {
		return
	}

	enc.writeString(name)
	enc.writeType(blockIONil)
}

func (enc *encoder) EncodeString(name, value string) {
	if enc.err != nil {
		return
	}

	enc.writeString(name)
	enc.writeType(blockIOString)

	// write value
	enc.writeString(value)
}

func (enc *encoder) EncodeInt(name string, value int64) {
	if enc.err != nil {
		return
	}

	enc.writeString(name)
	enc.writeType(blockIOInt)

	// write value
	enc.Output = binary.AppendVarint(enc.Output, value)
}

func (enc *encoder) EncodeUint(name string, value uint64) {
	if enc.err != nil {
		return
	}

	// not entirely needed yet, but seems like it would be easy to provide on top
	enc.writeString(name)
	enc.writeType(blockIOUint)

	// write value
	enc.Output = binary.AppendUvarint(enc.Output, value)
}

func (enc *encoder) EncodeBytes(name string, value []byte) {
	if enc.err != nil {
		return
	}

	enc.writeString(name)
	enc.writeType(blockIOBytes)

	// write value
	b := value
	enc.Output = binary.AppendUvarint(enc.Output, uint64(len(b)))
	enc.Output = append(enc.Output, b...)
}

func (enc *encoder) EncodeStruct(name string, value []byte) {
	if enc.err != nil {
		return
	}

	enc.writeString(name)
	enc.writeType(blockIOStruct)

	// write value
	b := value
	enc.Output = binary.AppendUvarint(enc.Output, uint64(len(b)))
	enc.Output = append(enc.Output, b...)
}

// encoder function generators
type encoderFunc func(enc *encoder, name string, elem reflect.Value)

var encoderFuncCache = &sync.Map{} // map[reflect.Type]encoderFunc

func getEncodeFunc(typ reflect.Type) encoderFunc {
	val, ok := encoderFuncCache.Load(typ)
	if ok {
		return val.(encoderFunc)
	}

	fn := buildEncodeFunc(typ)
	encoderFuncCache.Store(typ, fn)
	return fn
}

func buildEncodeFunc(typ reflect.Type) encoderFunc {
	switch typ.Kind() {
	case reflect.String:
		fallthrough
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fallthrough
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fallthrough
	case reflect.Struct:
		return buildEncodeScalarFunc(typ)
	case reflect.Pointer:
		return buildEncodeOrNilFunc(getEncodeFunc(typ.Elem()))
	case reflect.Slice:
		return buildEncodeSliceFunc(getEncodeFunc(typ.Elem()))
	default:
		panic(errors.New("not supported"))
	}
}

func buildEncodeScalarFunc(typ reflect.Type) encoderFunc {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	switch typ.Kind() {
	case reflect.String:
		return func(enc *encoder, name string, val reflect.Value) {
			val = reflect.Indirect(val)
			enc.EncodeString(name, val.String())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(enc *encoder, name string, val reflect.Value) {
			val = reflect.Indirect(val)
			enc.EncodeInt(name, val.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(enc *encoder, name string, val reflect.Value) {
			val = reflect.Indirect(val)
			enc.EncodeUint(name, val.Uint())
		}
	case reflect.Struct:
		if isByteString(typ) {
			// special treatment for ByteString's
			return func(enc *encoder, name string, val reflect.Value) {
				val = reflect.Indirect(val)
				buf := byteStringGetBuffer(&val)
				enc.EncodeBytes(name, *buf)
			}
		} else {
			return func(enc *encoder, name string, val reflect.Value) {
				val = reflect.Indirect(val)

				var b []byte
				b, enc.err = Marshal(val.Addr().Interface())
				if enc.err != nil {
					return
				}

				enc.EncodeStruct(name, b)
			}
		}
	default:
		panic(errors.New("not supported"))
	}
}

func buildEncodeOrNilFunc(fn encoderFunc) encoderFunc {
	return func(enc *encoder, name string, elem reflect.Value) {
		if elem.IsNil() {
			enc.EncodeNil(name)
		} else {
			fn(enc, name, elem)
		}
	}
}

func buildEncodeSliceFunc(encodeElem encoderFunc) encoderFunc {
	return func(enc *encoder, name string, val reflect.Value) {
		N := val.Len()

		//for i := 0; i < N; i++ {
		//	enc.EncodeValue(name, val.Index(i), typ.Elem())
		//	if enc.err != nil {
		//		return
		//	}
		//}

		// inlining this in an attempt to improve this benchmark
		/*
			First pass
			BenchmarkBiasSerializers/TC-ListDisks-json
			BenchmarkBiasSerializers/TC-ListDisks-json-16                	  253070	      4213 ns/op
			BenchmarkBiasSerializers/TC-ListDisks-block-io
			BenchmarkBiasSerializers/TC-ListDisks-block-io-16            	  229602	      5117 ns/op
			BenchmarkBiasSerializers/TC-ListOfPointers-json
			BenchmarkBiasSerializers/TC-ListOfPointers-json-16           	  288535	      3995 ns/op
			BenchmarkBiasSerializers/TC-ListOfPointers-block-io
			BenchmarkBiasSerializers/TC-ListOfPointers-block-io-16       	  257456	      4227 ns/op
			PASS

			Second pass, with encoderFunc:
			BenchmarkBiasSerializers/TC-ListDisks-json
			BenchmarkBiasSerializers/TC-ListDisks-json-16                	  267412	      4172 ns/op
			BenchmarkBiasSerializers/TC-ListDisks-block-io
			BenchmarkBiasSerializers/TC-ListDisks-block-io-16            	  206383	      5657 ns/op
			BenchmarkBiasSerializers/TC-ListOfPointers-json
			BenchmarkBiasSerializers/TC-ListOfPointers-json-16           	  277777	      4208 ns/op
			BenchmarkBiasSerializers/TC-ListOfPointers-block-io
			BenchmarkBiasSerializers/TC-ListOfPointers-block-io-16       	  230854	      4696 ns/op
			PASS
		*/
		// FIXME revisit this
		isPointer := val.Type().Kind() == reflect.Pointer
		if isPointer {
			encodeElem = buildEncodeOrNilFunc(encodeElem)
		}

		for i := 0; i < N; i++ {
			encodeElem(enc, name, val.Index(i))
		}
	}
}
