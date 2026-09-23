package apirt

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"
)

type contextKey string

const errGroupKey contextKey = "err-group"

func withErrGroup(ctx context.Context, g *errgroup.Group) context.Context {
	if g == nil {
		panic(errors.New("nil group"))
	}

	return context.WithValue(ctx, errGroupKey, g)
}

func GetErrGroup(ctx context.Context) *errgroup.Group {
	g := ctx.Value(errGroupKey)
	if g == nil {
		return nil
	}

	return g.(*errgroup.Group)
}
