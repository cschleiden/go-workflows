package tracing

import "fmt"

func WorkflowSpanName(workflowName string) string {
	return fmt.Sprintf("Workflow: %s", workflowName)
}
