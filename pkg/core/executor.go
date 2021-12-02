package core

import (
	"context"

	"github.com/cschleiden/go-dt/pkg/workflow"
)

type Executor interface {
}

type executorImpl struct {
}

func (e *executorImpl) ExecuteWorkflow(ctx context.Context, wf interface{}) {
	wfCtx := workflow.NewContext()
}
