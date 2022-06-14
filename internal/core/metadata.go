package core

type WorkflowInstanceMetadata map[string]string

func (wim WorkflowInstanceMetadata) Get(key string) string {
	return wim[key]
}

func (wim WorkflowInstanceMetadata) Set(key string, value string) {
	wim[key] = value
}

func (wim WorkflowInstanceMetadata) Keys() []string {
	r := make([]string, 0, len(wim))

	for k := range wim {
		r = append(r, k)
	}

	return r
}
