package sync

// func Test_FutureSelector_SelectWaits(t *testing.T) {
// 	ctx := context.Background()
// 	cs := newState()
// 	ctx = withCoState(ctx, cs)

// 	f := NewFuture(cs)

// 	reachedEnd := false

// 	cs.Run(ctx, func(ctx context.Context) {
// 		s := NewSelector(cs)

// 		s.AddFuture(f, func(f Future) {
// 			var r int
// 			err := f.Get(&r)
// 			require.Nil(t, err)

// 			require.Equal(t, 42, r)
// 		})

// 		// Wait for result
// 		s.Select()

// 		reachedEnd = true
// 	})

// 	cs.WaitUntilBlocked()

// 	require.False(t, reachedEnd)

// 	f.Set(func(v interface{}) error {
// 		x := reflect.ValueOf(v)
// 		x.Elem().Set(reflect.ValueOf(42))

// 		return nil
// 	})

// 	cs.Execute()

// 	require.True(t, reachedEnd)
// }

// func Test_FutureSelector_SelectWaitsWithSameOrder(t *testing.T) {
// 	ctx := context.Background()
// 	cs := newState()
// 	ctx = withCoState(ctx, cs)

// 	f := NewFuture(cs)
// 	f2 := NewFuture(cs)

// 	reachedEnd := false
// 	order := make([]int, 0)

// 	cs.Run(ctx, func(ctx context.Context) {
// 		s := NewSelector(cs)

// 		s.AddFuture(f, func(f Future) {
// 			var r int
// 			err := f.Get(&r)
// 			require.Nil(t, err)
// 			require.Equal(t, 42, r)
// 			order = append(order, 42)
// 		})

// 		s.AddFuture(f2, func(f Future) {
// 			var r int
// 			err := f.Get(&r)
// 			require.Nil(t, err)
// 			require.Equal(t, 23, r)
// 			order = append(order, 23)
// 		})

// 		// Wait for result
// 		s.Select()
// 		s.Select()

// 		reachedEnd = true
// 	})

// 	cs.WaitUntilBlocked()

// 	require.False(t, reachedEnd)

// 	f.Set(func(v interface{}) error {
// 		x := reflect.ValueOf(v)
// 		x.Elem().Set(reflect.ValueOf(42))

// 		return nil
// 	})

// 	f2.Set(func(v interface{}) error {
// 		x := reflect.ValueOf(v)
// 		x.Elem().Set(reflect.ValueOf(23))

// 		return nil
// 	})

// 	cs.Execute()

// 	require.True(t, cs.Finished())
// 	require.True(t, reachedEnd)
// 	require.Equal(t, []int{42, 23}, order)
// }

// func Test_FutureSelector_DefaultCase(t *testing.T) {
// 	ctx := context.Background()
// 	cs := newState()
// 	ctx = withCoState(ctx, cs)

// 	f := NewFuture(cs)

// 	defaultHandled := false
// 	reachedEnd := false

// 	cs.Run(ctx, func(ctx context.Context) {
// 		s := NewSelector(cs)

// 		s.AddFuture(f, func(_ Future) {
// 			require.Fail(t, "should not be called")
// 		})

// 		s.AddDefault(func() {
// 			defaultHandled = true
// 		})

// 		// Wait for result
// 		s.Select()

// 		reachedEnd = true
// 	})

// 	cs.WaitUntilBlocked()

// 	require.True(t, reachedEnd)
// 	require.True(t, defaultHandled)
// }

// func Test_ChannelSelector_Select(t *testing.T) {
// 	ctx := context.Background()
// 	cr := NewCoroutine()

// 	c := NewChannel()

// 	defaultHandled := false
// 	reachedEnd := false

// 	cr.Run(ctx, func(ctx context.Context) {
// 		s := NewSelector(cr)

// 		s.AddChannelReceive(c, func(_ Channel) {
// 			require.Fail(t, "should not be called")
// 		})

// 		// Wait for result
// 		s.Select()

// 		reachedEnd = true
// 	})

// 	cr.WaitUntilBlocked()

// 	require.True(t, reachedEnd)
// 	require.True(t, defaultHandled)
// }
