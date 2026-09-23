package apirt

import "context"

type RecvMessageStream[T any] interface {
	Recv(ctx context.Context) (T, error)
	Close() error
}

type SendMessageStream[T any] interface {
	Send(ctx context.Context, obj T) error
	Close() error
}

type MessageStream[Req any, Resp any] interface {
	RecvMessageStream[Req]
	SendMessageStream[Resp]
}
