package workflowstate

import (
	"github.com/cschleiden/go-workflows/internal/converter"
	"github.com/cschleiden/go-workflows/internal/payload"
	"github.com/cschleiden/go-workflows/internal/sync"
)

func ReceiveSignal(wf *WfState, name string, arg payload.Payload) {
	sc, ok := wf.signalChannels[name]
	if ok {
		sc.receive(arg)
		return
	}

	ps, ok := wf.pendingSignals[name]
	if !ok {
		ps = []payload.Payload{}
		wf.pendingSignals[name] = ps
	}

	wf.pendingSignals[name] = append(ps, arg)
}

func GetSignalChannel[T any](ctx sync.Context, wf *WfState, name string) sync.Channel[T] {
	// Check for existing channel, if exists return
	sc, ok := wf.signalChannels[name]
	if ok {
		return sc.channel.(sync.Channel[T])
	}

	// Otherwise, create new channel
	c := sync.NewBufferedChannel[T](100)

	converter := converter.GetConverter(ctx)

	// Add channel to map
	wf.signalChannels[name] = &signalChannel{
		receive: func(input payload.Payload) {
			var t T
			if err := converter.From(input, &t); err != nil {
				panic(err)
			}

			// Channel is buffered, so we can just send without waiting and potentially
			// blocking on a Yield.
			c.SendNonblocking(t)
		},
		channel: c,
	}

	// Check for any pending signals, if there are, send to the channel in reverse order
	pendingSignals, ok := wf.pendingSignals[name]
	if ok {
		for i := len(pendingSignals) - 1; i >= 0; i-- {
			payload := pendingSignals[i]

			var s T
			if err := converter.From(payload, &s); err != nil {
				panic(err)
			}

			c.Send(ctx, s)
		}

		delete(wf.pendingSignals, name)
	}

	return c
}
