package sync

type WaitGroup interface {
	Add(delta int)
	Done()
	Wait(ctx Context)
}

type waitGroup struct {
	n       int
	f       Future
	waiting bool
}

func NewWaitGroup() WaitGroup {
	return &waitGroup{
		f: NewFuture(),
	}
}

func (wg *waitGroup) Wait(ctx Context) {
	wg.waiting = true

	if err := wg.f.Get(ctx, nil); err != nil {
		panic(err)
	}
}

func (wg *waitGroup) Add(delta int) {
	wg.n += delta

	if wg.n < 0 {
		panic("negative WaitGroup counter")
	}

	if wg.waiting && delta > 0 && wg.n == delta {
		panic("WaitGroup misuse: Add called concurrently with Wait")
	}

	if wg.n > 0 || !wg.waiting {
		return
	}

	if wg.n == 0 {
		wg.f.Set(nil, nil)
	}
}

func (wg *waitGroup) Done() {
	wg.Add(-1)
}
