package apirt

import (
	"context"
	"net/http"

	"golang.org/x/sync/errgroup"
)

type OperationInput struct {
	Header http.Header
	Body   any // a pointer to a serializable object
}

type OperationOutput struct {
	StatusCode int
	Header     http.Header
	Body       any // a pointer to a serializable object
}

type OperationHook func(ctx context.Context, w ResponseWriter, req *Request, g *errgroup.Group) error

func invokeOperationWithHooks(
	ctx context.Context,
	w ResponseWriter,
	req *Request,
	g *errgroup.Group,
	middleware []OperationHook,
	impl OperationHook,
) (err error) {
	// execute hooks
	for _, mwf := range middleware {
		err = mwf(ctx, w, req, g)
		if err != nil {
			return
		}
	}

	// execute the implementation hook
	err = impl(ctx, w, req, g)
	return err
}
