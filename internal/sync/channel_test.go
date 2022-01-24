package sync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Channel_Unbuffered(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T, c *channel)
	}{
		{
			name: "Send_Blocks",
			fn: func(t *testing.T, c *channel) {
				cr := NewCoroutine(Background(), func(ctx Context) error {
					c.Send(ctx, 42)

					return nil
				})

				cr.Execute()

				require.False(t, cr.Finished())
				require.True(t, cr.Blocked())
			},
		},
		{
			name: "Receive_Blocks",
			fn: func(t *testing.T, c *channel) {
				cr := NewCoroutine(Background(), func(ctx Context) error {
					var r int
					c.Receive(ctx, &r)

					return nil
				})

				cr.Execute()

				require.False(t, cr.Finished())
				require.True(t, cr.Blocked())
			},
		},
		{
			name: "Receive_ToNil",
			fn: func(t *testing.T, c *channel) {
				cr := NewCoroutine(Background(), func(ctx Context) error {
					c.Receive(ctx, nil)
					return nil
				})
				cr.Execute()

				crSend := NewCoroutine(Background(), func(ctx Context) error {
					c.SendNonblocking(ctx, 42)

					return nil
				})
				crSend.Execute()

				cr.Execute()

				require.True(t, cr.Finished())
			},
		},
		{
			name: "Receive_BlocksUntilSend",
			fn: func(t *testing.T, c *channel) {
				var r int

				cr := NewCoroutine(Background(), func(ctx Context) error {
					more := c.Receive(ctx, &r)
					require.True(t, more)

					return nil
				})
				cr.Execute()

				require.True(t, cr.Blocked(), "coroutine should be blocked")

				crSend := NewCoroutine(Background(), func(ctx Context) error {
					c.SendNonblocking(ctx, 42)

					return nil
				})
				crSend.Execute()

				require.False(t, cr.Finished())
				require.True(t, cr.Blocked())

				cr.Execute()

				require.True(t, cr.Progress())
				require.True(t, cr.Finished())
				require.False(t, cr.Blocked())

				require.True(t, crSend.Finished())
				require.False(t, crSend.Blocked())

				require.Equal(t, 42, r)
			},
		},
		{
			name: "Receive_Closed",
			fn: func(t *testing.T, c *channel) {
				r := int(42)

				cr := NewCoroutine(Background(), func(ctx Context) error {
					more := c.Receive(ctx, &r)
					require.False(t, more)

					return nil
				})
				cr.Execute()

				require.True(t, cr.Blocked(), "coroutine should be blocked")

				crSend := NewCoroutine(Background(), func(ctx Context) error {
					c.Close()

					return nil
				})
				crSend.Execute()

				require.False(t, cr.Finished())
				require.True(t, cr.Blocked())

				cr.Execute()

				require.True(t, cr.Finished())
				require.False(t, cr.Blocked())

				require.True(t, crSend.Finished())
				require.False(t, crSend.Blocked())

				require.Zero(t, r)
			},
		},
		{
			name: "Send_BlocksUntilReceive",
			fn: func(t *testing.T, c *channel) {
				crSend := NewCoroutine(Background(), func(ctx Context) error {
					c.Send(ctx, 42)

					return nil
				})
				crSend.Execute()

				require.True(t, crSend.Blocked(), "coroutine should be blocked")

				var r int
				crReceive := NewCoroutine(Background(), func(ctx Context) error {
					more := c.Receive(ctx, &r)
					require.True(t, more)

					return nil
				})
				crReceive.Execute()

				require.False(t, crSend.Finished())
				require.True(t, crSend.Blocked())

				crSend.Execute()

				require.True(t, crSend.Finished())
				require.False(t, crSend.Blocked())

				require.True(t, crReceive.Finished())
				require.False(t, crReceive.Blocked())

				require.Equal(t, 42, r)
			},
		},
		{
			name: "SendNonblocking_DoesNotBlock",
			fn: func(t *testing.T, c *channel) {
				cr := NewCoroutine(Background(), func(ctx Context) error {
					r := c.SendNonblocking(ctx, 42)

					require.False(t, r)

					return nil
				})

				cr.Execute()

				require.True(t, cr.Finished())
				require.False(t, cr.Blocked())
			},
		},
		{
			name: "ReceiveNonblocking_DoesNotBlock",
			fn: func(t *testing.T, c *channel) {
				cr := NewCoroutine(Background(), func(ctx Context) error {
					r := c.SendNonblocking(ctx, 42)

					require.False(t, r)

					return nil
				})

				cr.Execute()

				require.True(t, cr.Finished())
				require.False(t, cr.Blocked())
			},
		},
		{
			name: "MultipleReceivesSends",
			fn: func(t *testing.T, c *channel) {

				ctx := Background()
				s := NewScheduler()

				var r int

				for i := 0; i < 10; i++ {
					s.NewCoroutine(ctx, func(ctx Context) error {
						var t int
						c.Receive(ctx, &t)
						r++

						return nil
					})
				}

				s.Execute(ctx)
				require.Equal(t, 0, r)

				for i := 0; i < 10; i++ {
					s.NewCoroutine(ctx, func(ctx Context) error {
						c.Send(ctx, 42)

						return nil
					})
				}

				for s.RunningCoroutines() > 0 {
					s.Execute(ctx)
				}

				require.Equal(t, 10, r)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChannel()
			tt.fn(t, c.(*channel))
		})
	}
}

func Test_Channel_Buffered(t *testing.T) {
	tests := []struct {
		name string
		size int
		fn   func(t *testing.T, c *channel)
	}{
		{
			name: "Send_Blocks",
			size: 3,
			fn: func(t *testing.T, c *channel) {
				ctx := Background()

				cs := NewCoroutine(ctx, func(ctx Context) error {
					c.Send(ctx, 1)
					c.Send(ctx, 2)
					c.Send(ctx, 3)

					return nil
				})
				cs.Execute()
				cs.Execute()
				cs.Execute()

				cr := NewCoroutine(ctx, func(ctx Context) error {
					var r int

					c.Receive(ctx, &r)
					require.Equal(t, 1, r)

					c.Receive(ctx, &r)
					require.Equal(t, 2, r)

					getCoState(ctx).Yield()

					c.Receive(ctx, &r)
					require.Equal(t, 3, r)

					c.Receive(ctx, &r)
					require.Equal(t, 0, r)

					return nil
				})

				cr.Execute()

				require.False(t, cr.Finished())
				require.True(t, cr.Blocked())

				c.Close()

				cr.Execute()

				require.True(t, cr.Finished())
				require.False(t, cr.Blocked())
			},
		},
		{
			name: "BufferedChannel_Send",
			size: 1,
			fn: func(t *testing.T, cs *channel) {
				ctx := Background()

				sentValue := false

				cr := NewCoroutine(ctx, func(ctx Context) error {
					cs.Send(ctx, 42)
					sentValue = true
					cs.Send(ctx, 23)

					return nil
				})

				cr.Execute()
				require.True(t, cr.Blocked()) // Blocking on second send
				require.True(t, sentValue)

				var r int
				crReceive := NewCoroutine(ctx, func(ctx Context) error {
					for {
						cs.Receive(ctx, &r)
						getCoState(ctx).Yield()
					}
				})

				crReceive.Execute()
				require.Equal(t, 42, r)

				cr.Execute()
				require.True(t, cr.Finished())

				crReceive.Execute()
				require.Equal(t, 23, r)
			},
		},
		{
			name: "BufferedChannel_Receive_ToNil",
			size: 1,
			fn: func(t *testing.T, c *channel) {
				cr := NewCoroutine(Background(), func(ctx Context) error {
					c.Send(ctx, 42)

					c.Receive(ctx, nil)

					return nil
				})
				cr.Execute()

				require.NoError(t, cr.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewBufferedChannel(tt.size)
			tt.fn(t, c.(*channel))
		})
	}
}
