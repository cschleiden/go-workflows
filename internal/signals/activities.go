package signals

import (
	"context"
)

type Signaler interface {
	SignalWorkflow(ctx context.Context, instanceID string, name string, arg interface{}) error
}

type Activities struct {
	Signaler Signaler
}

func (a *Activities) DeliverWorkflowSignal(ctx context.Context, instanceID, signalName string, arg interface{}) error {
	return a.Signaler.SignalWorkflow(ctx, instanceID, signalName, arg)
}
