package worker

import workflowinternal "github.com/cschleiden/go-workflows/internal/workflow"

type RegisterOption workflowinternal.RegisterOption

type registerOptions []RegisterOption

func (opts registerOptions) asInternalOptions() []workflowinternal.RegisterOption {
	repacked := make([]workflowinternal.RegisterOption, len(opts))
	for i, opt := range opts {
		repacked[i] = workflowinternal.RegisterOption(opt)
	}
	return repacked
}

func WithName(name string) RegisterOption {
	return workflowinternal.WithName(name)
}
