package converter

import "encoding/json"

type Converter interface {
	To(v interface{}) ([]byte, error)
	From(data []byte, v interface{}) error
}

var DefaultConverter Converter = &jsonConverter{}

type jsonConverter struct{}

func (jc *jsonConverter) To(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jc *jsonConverter) From(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
