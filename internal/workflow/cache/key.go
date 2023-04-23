package cache

import (
	"github.com/cschleiden/go-workflows/internal/core"
)

func getKey(instance *core.WorkflowInstance) string {
	return instance.InstanceID
}
