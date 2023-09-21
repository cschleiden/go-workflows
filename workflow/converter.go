package workflow

import (
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/converter"
	iconverter "github.com/cschleiden/go-workflows/internal/converter"
)

type (
	Converter = converter.Converter
	Payload   = payload.Payload
)

var DefaultConverter = converter.DefaultConverter

func WithConverter(ctx Context, c Converter) Context {
	return iconverter.WithConverter(ctx, c)
}

func GetConverter(ctx Context) Converter {
	return iconverter.Converter(ctx)
}
