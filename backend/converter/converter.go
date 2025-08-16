package converter

import (
	"github.com/cschleiden/go-workflows/backend/payload"
)

type Converter interface {
	// To converts the given value to a payload
	To(v any) (payload.Payload, error)

	// From converts the given payload to a value
	From(data payload.Payload, v any) error
}

var DefaultConverter Converter = &jsonConverter{}
