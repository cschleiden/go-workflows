package converter

import (
	"reflect"

	"github.com/cschleiden/go-dt/internal/payload"
	"github.com/pkg/errors"
)

type Converter interface {
	To(v interface{}) (payload.Payload, error)
	From(data payload.Payload, v interface{}) error
}

var DefaultConverter Converter = &jsonConverter{}

func ArgsToInputs(c Converter, args ...interface{}) ([]payload.Payload, error) {
	inputs := make([]payload.Payload, 0)

	for _, arg := range args {
		input, err := c.To(arg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert activity input")
		}
		inputs = append(inputs, input)
	}

	return inputs, nil
}

func AssignValue(c Converter, v interface{}, vptr interface{}) {
	vvptr := reflect.ValueOf(vptr)

	if vvptr.Kind() != reflect.Ptr {
		panic("vptr needs to be a pointer")
	}

	if v == nil {
		vvptr.Elem().Set(reflect.Zero(vvptr.Elem().Type()))
		return
	}

	// Try converting value first
	if vp, ok := v.(payload.Payload); ok {
		err := c.From(vp, vptr)
		if err != nil {
			panic(err)
		}
	} else {
		// TODO: Assert that values can be assigned
		vvptr.Elem().Set(reflect.ValueOf(v))
	}
}
