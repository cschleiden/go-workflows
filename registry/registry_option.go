package registry

type RegisterOption interface {
	applyRegisterOption(registerConfig) registerConfig
}

type registerOptions []RegisterOption

func (opts registerOptions) applyRegisterOptions(cfg registerConfig) registerConfig {
	for _, opt := range opts {
		cfg = opt.applyRegisterOption(cfg)
	}
	return cfg
}

type registerOptionFunc func(registerConfig) registerConfig

func (f registerOptionFunc) applyRegisterOption(cfg registerConfig) registerConfig {
	return f(cfg)
}

func WithName(name string) RegisterOption {
	return registerOptionFunc(func(cfg registerConfig) registerConfig {
		cfg.Name = name
		return cfg
	})
}
