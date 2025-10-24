# Workflow Info Sample

This sample demonstrates how to access workflow information during workflow execution, specifically the history length.

## What it demonstrates

- Using `workflow.InstanceExecutionDetails(ctx)` to access workflow metadata
- Tracking how the workflow history grows as events are added
- Accessing the `HistoryLength` field of `WorkflowInstanceExecutionDetails`

## Running the sample

```bash
go run .
```

## Expected Output

You should see log messages showing the history length increasing as the workflow executes:

```
Workflow started historyLength=2
Activity executed
After activity execution historyLength=5
Activity executed
After second activity historyLength=8
Workflow completed successfully!
```

## How it works

The `WorkflowInstanceExecutionDetails` struct contains information about the current workflow execution. Currently it provides:

- `HistoryLength`: The number of events in the workflow history at the current point in execution

The history length includes all events that have been added to the workflow's event history, including:
- WorkflowExecutionStarted
- WorkflowTaskStarted
- ActivityScheduled
- ActivityCompleted
- TimerScheduled
- TimerFired
- And other workflow events

This can be useful for:
- Monitoring workflow complexity
- Making decisions based on how far the workflow has progressed
- Implementing custom limits or checkpointing logic
- Debugging and understanding workflow execution

## Future extensions

The `WorkflowInstanceExecutionDetails` struct is designed to be extensible. Future additions might include:
- Execution duration
- Number of activities executed
- Number of retries
- Custom metadata
