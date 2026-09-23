package blockio

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// encodableField contains enough
// information for us to encode/decode a field
// in a struct
type encodableField struct {
	EncodedName string
	FieldIndex  int
	Field       reflect.StructField
}

var structTypeFieldCache = &sync.Map{} // map[reflect.Type][]encodableField

// returns the encodableField's for the given reflect.Type
// if it isn't in the cache, it will be loaded into the cache
func cachedStructTypeFields(typ reflect.Type) (fields []encodableField) {
	val, ok := structTypeFieldCache.Load(typ)
	if ok {
		fields = val.([]encodableField)
		return
	}

	fields = structTypeFields(typ)
	structTypeFieldCache.Store(typ, fields)
	return fields
}

// use cachedStructTypeFields instead
func structTypeFields(typ reflect.Type) (fields []encodableField) {
	if typ.Kind() != reflect.Struct {
		panic(fmt.Errorf("expected a struct, got a: %s (%s)", typ.Kind().String(), typ.String()))
	}

	for i := 0; i < typ.NumField(); i++ {
		structField := typ.Field(i)
		encField := encodableField{
			EncodedName: encodedFieldName(structField),
			FieldIndex:  i,
			Field:       structField,
		}

		fields = append(fields, encField)
	}

	return
}

func encodedFieldName(sf reflect.StructField) string {
	json := sf.Tag.Get("json")
	if json == "" {
		return sf.Name
	}

	name, _, _ := strings.Cut(json, ",")
	if name == "" {
		return sf.Name
	}

	return name
}

func typIndirect(typ reflect.Type) reflect.Type {
	if typ.Kind() == reflect.Pointer {
		return typ.Elem()
	}
	return typ
}
