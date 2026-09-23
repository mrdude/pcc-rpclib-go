package blockio

import "reflect"

// returns true if typ is an apirt.ByteString
func isByteString(typ reflect.Type) bool {

	// old implementation
	//var byteStringType = reflect.TypeOf(apirt.ByteString{})
	//return typ.AssignableTo(byteStringType)

	// checking the type this way allows us to avoid importing apirt,
	// which would lead to a circular import
	// TODO this is kinda hacky, should revisit
	if typ.String() == "apirt.ByteString" {
		return true
	}
	return false
}

// returns a pointer to apirt.ByteString's Buf
func byteStringGetBuffer(val *reflect.Value) *[]byte {
	// apirt.ByteString's byte string field name
	// TODO if this field name changes, this will break -- add tests
	const byteStringBufferFieldName = "Buf"
	field := val.FieldByName(byteStringBufferFieldName)
	return field.Addr().Interface().(*[]byte)
}
