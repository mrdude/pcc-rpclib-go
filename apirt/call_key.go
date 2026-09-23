package apirt

import "context"

const callKeyContextKey = contextKey("call-key")

type CallKey struct {
	ApiName       string
	OperationName string
}

func WithCallKey(ctx context.Context, key CallKey) context.Context {
	return context.WithValue(ctx, callKeyContextKey, &key)
}

func GetCallKey(ctx context.Context) CallKey {
	ser := ctx.Value(ctx)
	if ser == nil {
		return CallKey{}
	}

	return *ser.(*CallKey)
}
