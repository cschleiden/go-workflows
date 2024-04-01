package tracing

// workflowspan is a span that can have a custom start/end time
type workflowspan struct {
}

// TODO
// 1. When workflow is created, start a new workflow root span.
// 2. Store that span in the workflow's metadata
// 3. Every time we execute that workflow, be it a new execution or a replay, ensure this span is set as the active span
// 4. When the workflow ends, end the span
// 5. Keep the task execution spans as separate ones.
