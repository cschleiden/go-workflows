package core

type WorkflowMetadata map[string]string

func (wim WorkflowMetadata) Get(key string) string {
	return wim[key]
}

func (wim WorkflowMetadata) Set(key string, value string) {
	wim[key] = value
}

func (wim WorkflowMetadata) Keys() []string {
	r := make([]string, 0, len(wim))

	for k := range wim {
		r = append(r, k)
	}

	return r
}
