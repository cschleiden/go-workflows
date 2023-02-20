package converter

import "github.com/cschleiden/go-workflows/internal/sync"

type converterKey struct{}

func WithConverter(ctx sync.Context, converter Converter) sync.Context {
	return sync.WithValue(ctx, converterKey{}, converter)
}

func GetConverter(ctx sync.Context) Converter {
	return ctx.Value(converterKey{}).(Converter)
}
