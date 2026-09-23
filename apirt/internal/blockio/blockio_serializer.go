// Package blockio implements the blockIO serializer
// TODO explain rationale behind this package
package blockio

import (
	"errors"
	"fmt"
	"reflect"
)

func Marshal(v any) ([]byte, error) {
	if reflect.TypeOf(v).Kind() != reflect.Pointer {
		return nil, errors.New("top-level object must be pointer")
	}

	/*
		values to handle:
			- structs (top level and embedded)
			- lists of structs
			- nil
			- string
			- int64, int32 (just include all ints, signed)
			- ByteString
	*/

	// iterate through each field, encoding each
	enc := &encoder{}
	val := reflect.ValueOf(v).Elem()
	typ := typIndirect(reflect.TypeOf(v).Elem())
	typeFields := cachedStructTypeFields(typ)
	for i := range typeFields {
		encField := &typeFields[i]
		valField := val.Field(encField.FieldIndex)

		enc.EncodeValue(encField.EncodedName, valField, encField.Field.Type)
		if err := enc.Err(); err != nil {
			return nil, err
		}
	}

	return enc.Output, nil
}

func Unmarshal(data []byte, v any) error {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	// follow pointers and interfaces
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Interface {
		if val.IsNil() {
			return fmt.Errorf("top-level object must be non-null")
		}

		val = val.Elem()
		typ = val.Type()
	}

	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("top-level object must be a struct, pointer to a struct, or an interface; instead got %s", reflect.TypeOf(v).String())
	}

	// iterate all fields, and place them into a map
	fieldMap := make(map[string]*reflect.Value)
	typeFields := cachedStructTypeFields(typ)
	for i := range typeFields {
		encField := &typeFields[i]
		valField := val.Field(encField.FieldIndex)

		fieldMap[encField.EncodedName] = &valField
	}

	// decode the fields, and place them into the object
	dec := &decoder{
		Input: data,
		Pos:   0,
	}
	for !dec.Eof() {
		encodedFieldName, err := dec.ReadString()
		if err != nil {
			return err
		}

		valField, ok := fieldMap[encodedFieldName]
		if !ok {
			return fmt.Errorf("unknown field: %s", encodedFieldName)
		}

		typ, err := dec.ReadFieldType()
		if err != nil {
			return err
		}

		switch typ {
		case blockIONil:
			valField.Set(reflect.Zero(valField.Type()))
		case blockIOString:
			val, err := dec.ReadString()
			if err != nil {
				return err
			}

			switch {
			case valField.Type().Kind() == reflect.Pointer && valField.Type().Elem().Kind() == reflect.String:
				v := reflect.ValueOf(&val)
				valField.Set(v)
			case valField.Type().Kind() == reflect.String:
				valField.SetString(val)
			default:
				return errors.New("wrong physical type for this wire type")
			}
		case blockIOInt:
			val, err := dec.ReadInt()
			if err != nil {
				return err
			}

			valField.SetInt(val) // TODO check so we don't panic
		case blockIOUint:
			val, err := dec.ReadUint()
			if err != nil {
				return err
			}

			valField.SetUint(val) // TODO check so we don't panic
		case blockIOBytes:
			var buf []byte
			buf, err = dec.ReadByteString()
			if err != nil {
				return err
			}

			field := byteStringGetBuffer(valField)
			*field = buf
		case blockIOStruct:
			bs, err := dec.ReadByteString()
			if err != nil {
				return err
			}

			switch {
			case valField.Type().Kind() == reflect.Slice && valField.Type().Elem().Kind() == reflect.Struct:
				// specifically handle repeated elements of structs

				obj := reflect.New(valField.Type().Elem()) // pointer to struct

				err = Unmarshal(bs, obj.Interface())
				if err != nil {
					return err
				}

				valField.Set(reflect.Append(*valField, obj.Elem()))
			case valField.Type().Kind() == reflect.Slice && valField.Type().Elem().Kind() == reflect.Pointer && valField.Type().Elem().Elem().Kind() == reflect.Struct:
				// slices of pointers to structs
				obj := reflect.New(valField.Type().Elem().Elem()) // pointer to struct

				err = Unmarshal(bs, obj.Interface())
				if err != nil {
					return err
				}

				valField.Set(reflect.Append(*valField, obj))
			case valField.Type().Kind() == reflect.Struct:
				err = Unmarshal(bs, valField.Addr().Interface())
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("wrong physical type for this wire type: %s", valField.Type().String())
			}
		default:
			return fmt.Errorf("unknown field type: %d, %d/%d", typ, dec.Pos, len(dec.Input))
		}
	}

	return nil
}
