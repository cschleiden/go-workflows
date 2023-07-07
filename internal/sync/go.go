package sync

func Go(ctx Context, f func(ctx Context)) {
	cs := getCoState(ctx)

	cs.creator.NewCoroutine(ctx, func(ctx Context) error {
		f(ctx)

		return nil
	})
}
