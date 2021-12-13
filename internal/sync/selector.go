package sync

import "context"

type Selector interface {
	AddFuture(f Future, handler func(f Future))
	Select(ctx context.Context)
}

func NewSelector() Selector {
	return &selector{
		cases: make([]selectorCase, 0),
	}
}

type selector struct {
	cases []selectorCase
}

func (s *selector) AddFuture(f Future, handler func(f Future)) {
	s.cases = append(s.cases, &futureCase{
		f:  f.(*futureImpl),
		fn: handler,
	})
}

func (s *selector) Select(ctx context.Context) {
	for {
		// Is any case ready?
		for i, c := range s.cases {
			if c.Ready() {
				c.Handle()
				// Remove successful case
				s.cases = append(s.cases[:i], s.cases[i+1:]...)
				return
			}
		}

		// else, yield and wait for result
		getCoState(ctx).Yield()
	}
}

type selectorCase interface {
	Ready() bool
	Handle()
}

var _ = selectorCase(&futureCase{})

type futureCase struct {
	f  *futureImpl
	fn func(Future)
}

func (fc *futureCase) Ready() bool {
	return fc.f.Ready()
}

func (fc *futureCase) Handle() {
	fc.fn(fc.f)
}
