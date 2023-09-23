package contextvalue

import (
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/internal/sync"
)

type converterKey struct{}

func WithConverter(ctx sync.Context, converter converter.Converter) sync.Context {
	return sync.WithValue(ctx, converterKey{}, converter)
}

func Converter(ctx sync.Context) converter.Converter {
	return ctx.Value(converterKey{}).(converter.Converter)
}
