package worker

import workflowinternal "github.com/cschleiden/go-workflows/internal/workflow"

type RegisterOption interface {
	applyRegisterOption(workflowinternal.RegisterConfig) workflowinternal.RegisterConfig
}

type registerOptions []RegisterOption

func (opts registerOptions) applyRegisterOptions(cfg workflowinternal.RegisterConfig) workflowinternal.RegisterConfig {
	for _, opt := range opts {
		cfg = opt.applyRegisterOption(cfg)
	}
	return cfg
}

type registerOptionFunc func(workflowinternal.RegisterConfig) workflowinternal.RegisterConfig

func (f registerOptionFunc) applyRegisterOption(cfg workflowinternal.RegisterConfig) workflowinternal.RegisterConfig {
	return f(cfg)
}

func WithName(name string) RegisterOption {
	return registerOptionFunc(func(cfg workflowinternal.RegisterConfig) workflowinternal.RegisterConfig {
		cfg.Name = name
		return cfg
	})
}
