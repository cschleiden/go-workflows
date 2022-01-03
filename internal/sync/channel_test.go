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
				cr := NewCoroutine(Background(), func(ctx Context) {
					c.Send(ctx, 42)
				})

				cr.Execute()

				require.False(t, cr.Finished())
				require.True(t, cr.Blocked())
			},
		},
		{
			name: "Receive_Blocks",
			fn: func(t *testing.T, c *channel) {
				cr := NewCoroutine(Background(), func(ctx Context) {
					var r int
					c.Receive(ctx, &r)
				})

				cr.Execute()

				require.False(t, cr.Finished())
				require.True(t, cr.Blocked())
			},
		},
		{
			name: "Receive_BlocksUntilSend",
			fn: func(t *testing.T, c *channel) {
				var r int

				cr := NewCoroutine(Background(), func(ctx Context) {
					more := c.Receive(ctx, &r)
					require.True(t, more)
				})
				cr.Execute()

				require.True(t, cr.Blocked(), "coroutine should be blocked")

				crSend := NewCoroutine(Background(), func(ctx Context) {
					c.SendNonblocking(ctx, 42)
				})
				crSend.Execute()

				require.False(t, cr.Finished())
				require.True(t, cr.Blocked())

				cr.Execute()

				require.True(t, cr.Finished())
				require.False(t, cr.Blocked())

				require.True(t, crSend.Finished())
				require.False(t, crSend.Blocked())

				require.Equal(t, 42, r)
			},
		},
		{
			name: "Send_BlocksUntilReceive",
			fn: func(t *testing.T, c *channel) {
				crSend := NewCoroutine(Background(), func(ctx Context) {
					c.Send(ctx, 42)
				})
				crSend.Execute()

				require.True(t, crSend.Blocked(), "coroutine should be blocked")

				var r int
				crReceive := NewCoroutine(Background(), func(ctx Context) {
					more := c.Receive(ctx, &r)
					require.True(t, more)
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
				cr := NewCoroutine(Background(), func(ctx Context) {
					r := c.SendNonblocking(ctx, 42)

					require.False(t, r)
				})

				cr.Execute()

				require.True(t, cr.Finished())
				require.False(t, cr.Blocked())
			},
		},
		{
			name: "ReceiveNonblocking_DoesNotBlock",
			fn: func(t *testing.T, c *channel) {
				cr := NewCoroutine(Background(), func(ctx Context) {
					r := c.SendNonblocking(ctx, 42)

					require.False(t, r)
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
					s.NewCoroutine(ctx, func(ctx Context) {
						var t int
						c.Receive(ctx, &t)
						r++
					})
				}

				s.Execute(ctx)
				require.Equal(t, 0, r)

				for i := 0; i < 10; i++ {
					s.NewCoroutine(ctx, func(ctx Context) {
						c.Send(ctx, 42)
					})
				}

				for s.RunningCoroutines() > 0 {
					s.Execute(ctx)
				}

				require.Equal(t, 10, r)
			},
		},
		{
			name: "BufferedChannel_Send",
			fn: func(t *testing.T, c *channel) {
				ctx := Background()
				cs := NewBufferedChannel(1)

				sentValue := false

				cr := NewCoroutine(ctx, func(ctx Context) {
					cs.Send(ctx, 42)
					sentValue = true
					cs.Send(ctx, 23)
				})

				cr.Execute()
				require.True(t, cr.Blocked()) // Blocking on second send
				require.True(t, sentValue)

				var r int
				crReceive := NewCoroutine(ctx, func(ctx Context) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChannel()
			tt.fn(t, c.(*channel))
		})
	}
}
