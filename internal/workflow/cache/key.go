package cache

import (
	"fmt"

	"github.com/ticctech/go-workflows/internal/core"
)

func getKey(instance *core.WorkflowInstance) string {
	return fmt.Sprintf("%s-%s", instance.InstanceID, instance.ExecutionID)
}
