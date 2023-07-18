package workflow

import (
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/payload"
)

type (
	Converter = converter.Converter
	Payload   = payload.Payload
)

var DefaultConverter = converter.DefaultConverter

func WithConverter(ctx Context, c Converter) Context {
	return converter.WithConverter(ctx, c)
}

func GetConverter(ctx Context) Converter {
	return converter.GetConverter(ctx)
}
