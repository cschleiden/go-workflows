package converter

import (
	"reflect"

	"github.com/cschleiden/go-dt/internal/payload"
)

type Converter interface {
	To(v interface{}) (payload.Payload, error)
	From(data payload.Payload, v interface{}) error
}

var DefaultConverter Converter = &jsonConverter{}

func AssignValue(c Converter, v interface{}, vptr interface{}) error {
	vvptr := reflect.ValueOf(vptr)

	if vvptr.Kind() != reflect.Ptr {
		panic("vptr needs to be a pointer")
	}

	if v == nil {
		vvptr.Elem().Set(reflect.Zero(vvptr.Elem().Type()))
		return nil
	}

	// Try converting value first
	if vp, ok := v.(payload.Payload); ok {
		if vp == nil {
			vvptr.Elem().Set(reflect.Zero(vvptr.Elem().Type()))
			return nil
		}

		return c.From(vp, vptr)
	} else {
		// TODO: Assert that values can be assigned
		vvptr.Elem().Set(reflect.ValueOf(v))
	}

	return nil
}
