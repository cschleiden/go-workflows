package core

type Future interface {
	Set(v interface{})
	Get() (interface{}, error)
}

type futureImpl struct {
	c chan interface{}
}

func NewFuture() Future {
	return &futureImpl{
		c: make(chan interface{}, 1),
	}
}

func (f *futureImpl) Set(v interface{}) {
	f.c <- v
}

func (f *futureImpl) Get() (interface{}, error) {
	v := <-f.c
	return v, nil
}
