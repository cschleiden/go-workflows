package converter

import (
	"encoding/json"

	"github.com/cschleiden/go-workflows/backend/payload"
)

type jsonConverter struct{}

func (jc *jsonConverter) To(v any) (payload.Payload, error) {
	return json.Marshal(v)
}

func (jc *jsonConverter) From(data payload.Payload, vptr any) error {
	return json.Unmarshal(data, vptr)
}
