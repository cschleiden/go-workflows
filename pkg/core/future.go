package core

type Future interface {
	Set(v interface{})
	Get() (interface{}, error)
}

var _ Future = &futureImpl{}

type futureImpl struct {
	c chan interface{}
}

func (f *futureImpl) Set(v interface{}) {
	f.c <- v
}

func (f *futureImpl) Get() (interface{}, error) {
	v := <-f.c
	return v, nil
}
