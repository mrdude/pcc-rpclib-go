package blockio

type fieldType byte

const (
	blockIONil    fieldType = 0
	blockIOString fieldType = 1
	blockIOInt    fieldType = 2
	blockIOUint   fieldType = 3
	blockIOBytes  fieldType = 4
	blockIOStruct fieldType = 5
)

func (typ fieldType) String() string {
	switch typ {
	case blockIONil:
		return "nil"
	case blockIOString:
		return "string"
	case blockIOInt:
		return "int"
	case blockIOUint:
		return "uint"
	case blockIOBytes:
		return "bytes"
	case blockIOStruct:
		return "struct"
	default:
		return "<unknown>"
	}
}
