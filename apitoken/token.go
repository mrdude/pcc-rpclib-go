package apitoken

var defaultJsonCoder = &defaultCoder{}

func DecodeJsonToken(encodedToken *string, target any) (isEmpty bool, err error) {
	return defaultJsonCoder.DecodeJsonToken(encodedToken, target)
}

func EncodeJsonToken(target any) *string {
	return defaultJsonCoder.EncodeJsonToken(target)
}
