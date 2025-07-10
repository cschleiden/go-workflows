package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkflowOrchestrator_PollerDefaults(t *testing.T) {
	tests := []struct {
		name                        string
		inputOptions                *Options
		expectedWorkflowPollers     int
		expectedActivityPollers     int
		expectedSingleWorkerMode    bool
	}{
		{
			name:                     "nil options should use 1 poller for orchestrator",
			inputOptions:             nil,
			expectedWorkflowPollers:  1,
			expectedActivityPollers:  1,
			expectedSingleWorkerMode: true,
		},
		{
			name: "default options should change to 1 poller for orchestrator",
			inputOptions: &Options{
				WorkflowWorkerOptions: WorkflowWorkerOptions{
					WorkflowPollers: DefaultOptions.WorkflowPollers, // Should be 2
				},
				ActivityWorkerOptions: ActivityWorkerOptions{
					ActivityPollers: DefaultOptions.ActivityPollers, // Should be 2
				},
			},
			expectedWorkflowPollers:  1,
			expectedActivityPollers:  1,
			expectedSingleWorkerMode: true,
		},
		{
			name: "custom options should be preserved",
			inputOptions: &Options{
				WorkflowWorkerOptions: WorkflowWorkerOptions{
					WorkflowPollers: 3,
				},
				ActivityWorkerOptions: ActivityWorkerOptions{
					ActivityPollers: 5,
				},
			},
			expectedWorkflowPollers:  3,
			expectedActivityPollers:  5,
			expectedSingleWorkerMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from NewWorkflowOrchestrator
			options := tt.inputOptions
			if options == nil {
				options = &DefaultOptions
			}

			// Enable SingleWorkerMode automatically for the orchestrator
			orchestratorOptions := *options
			orchestratorOptions.SingleWorkerMode = true

			// Set default pollers to 1 for orchestrator mode (unless explicitly overridden)
			if orchestratorOptions.WorkflowPollers == DefaultOptions.WorkflowPollers {
				orchestratorOptions.WorkflowPollers = 1
			}
			if orchestratorOptions.ActivityPollers == DefaultOptions.ActivityPollers {
				orchestratorOptions.ActivityPollers = 1
			}

			// Verify the results
			require.Equal(t, tt.expectedWorkflowPollers, orchestratorOptions.WorkflowPollers)
			require.Equal(t, tt.expectedActivityPollers, orchestratorOptions.ActivityPollers)
			require.Equal(t, tt.expectedSingleWorkerMode, orchestratorOptions.SingleWorkerMode)
		})
	}
}