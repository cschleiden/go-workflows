package workflow

type RegisterOption interface {
	applyRegisterOption(RegisterConfig) RegisterConfig
}

type registerOptions []RegisterOption

func (opts registerOptions) applyRegisterOptions(cfg RegisterConfig) RegisterConfig {
	for _, opt := range opts {
		cfg = opt.applyRegisterOption(cfg)
	}
	return cfg
}

type registerOptionFunc func(RegisterConfig) RegisterConfig

func (f registerOptionFunc) applyRegisterOption(cfg RegisterConfig) RegisterConfig {
	return f(cfg)
}

func WithName(name string) RegisterOption {
	return registerOptionFunc(func(cfg RegisterConfig) RegisterConfig {
		cfg.Name = name
		return cfg
	})
}
