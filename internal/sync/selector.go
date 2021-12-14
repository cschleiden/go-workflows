package sync

type Selector interface {
	AddFuture(f Future, handler func(f Future))

	AddDefault(handler func())

	Select()
}

func NewSelector(cr Coroutine) Selector {
	return &selector{
		cr:    cr,
		cases: make([]selectorCase, 0),
	}
}

type selector struct {
	cr    Coroutine
	cases []selectorCase

	defaultFunc func()
}

func (s *selector) AddFuture(f Future, handler func(f Future)) {
	s.cases = append(s.cases, &futureCase{
		f:  f.(*futureImpl),
		fn: handler,
	})
}

func (s *selector) AddDefault(handler func()) {
	s.defaultFunc = handler
}

func (s *selector) Select() {
	for {
		// Is any case ready?
		for i, c := range s.cases {
			if c.Ready() {
				c.Handle()
				// Remove handled case
				s.cases = append(s.cases[:i], s.cases[i+1:]...)
				return
			}
		}

		if s.defaultFunc != nil {
			s.defaultFunc()
			return
		}

		// else, yield and wait for result
		s.cr.Yield()
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
