package converter

import (
	"encoding/json"

	"github.com/cschleiden/go-dt/internal/payload"
)

type jsonConverter struct{}

func (jc *jsonConverter) To(v interface{}) (payload.Payload, error) {
	return json.Marshal(v)
}

func (jc *jsonConverter) From(data payload.Payload, vptr interface{}) error {
	return json.Unmarshal(data, vptr)
}