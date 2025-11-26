package sync

// ErrGroup provides a way to run functions concurrently and collect the first error.
//
// It is conceptually similar to golang.org/x/sync/errgroup.Group but adapted to the
// workflow scheduler and Context. It cancels the derived Context when the first function
// returns a non-nil error. Wait waits for all functions to finish and returns the first
// error that was observed. If the Context passed to Wait is canceled before completion,
// Wait returns that context error instead.
type ErrGroup interface {
	// Go starts the given function in a new workflow coroutine.
	// The started coroutine receives the group's derived Context, which is canceled when the
	// first function returns a non-nil error.
	Go(f func(Context) error)

	// Wait waits for all launched functions to complete. It returns the first non-nil error
	// returned by any function. If the provided ctx is canceled before completion, the context
	// error is returned.
	Wait(ctx Context) error
}

type errGroup struct {
	// count of running functions
	n int

	// future that gets set when the count drops to zero
	done SettableFuture[struct{}]

	// first error encountered
	firstErr error

	// cancel the derived context
	cancel CancelFunc

	// context associated with this group (child of parent)
	ctx Context

	// track if Wait was called to detect certain misuses (optional)
	waiting bool

	// coroutine creator captured from the parent context when the group is created
	creator CoroutineCreator
}

// WithErrGroup creates a child Context and an ErrGroup. The returned Context is canceled
// automatically when any function started with g.Go returns a non-nil error.
func WithErrGroup(parent Context) (Context, ErrGroup) {
	ctx, cancel := WithCancel(parent)
	cs := getCoState(parent)
	return ctx, &errGroup{
		done:    NewFuture[struct{}](),
		cancel:  cancel,
		ctx:     ctx,
		creator: cs.creator,
	}
}

func (g *errGroup) Go(f func(Context) error) {
	g.n += 1

	g.creator.NewCoroutine(g.ctx, func(ctx Context) error {
		// Execute user function
		if err := f(ctx); err != nil {
			if g.firstErr == nil {
				g.firstErr = err
				// cancel group context on first error
				if g.cancel != nil {
					g.cancel()
				}
			}
		}

		g.n -= 1
		if g.n < 0 {
			panic("negative ErrGroup counter")
		}
		if g.n == 0 {
			g.done.Set(struct{}{}, nil)
		}

		return nil
	})
}

func (g *errGroup) Wait(ctx Context) error {
	g.waiting = true

	if g.n == 0 {
		return g.firstErr
	}

	if _, err := g.done.Get(ctx); err != nil {
		return err
	}
	return g.firstErr
}
