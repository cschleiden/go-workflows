package backend

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithWorkflowLockTimeout(t *testing.T) {
	timeout := 5 * time.Minute
	option := WithWorkflowLockTimeout(timeout)

	opts := ApplyOptions(option)

	require.Equal(t, timeout, opts.WorkflowLockTimeout)
}

func TestWithActivityLockTimeout(t *testing.T) {
	timeout := 3 * time.Minute
	option := WithActivityLockTimeout(timeout)

	opts := ApplyOptions(option)

	require.Equal(t, timeout, opts.ActivityLockTimeout)
}

func TestWithWorkflowAndActivityLockTimeout(t *testing.T) {
	workflowTimeout := 2 * time.Minute
	activityTimeout := 4 * time.Minute

	opts := ApplyOptions(
		WithWorkflowLockTimeout(workflowTimeout),
		WithActivityLockTimeout(activityTimeout),
	)

	require.Equal(t, workflowTimeout, opts.WorkflowLockTimeout)
	require.Equal(t, activityTimeout, opts.ActivityLockTimeout)
}

func TestDefaultValues(t *testing.T) {
	opts := ApplyOptions()

	// Verify default values are preserved when no options are provided
	require.Equal(t, time.Minute, opts.WorkflowLockTimeout)
	require.Equal(t, time.Minute*2, opts.ActivityLockTimeout)
}

// TestIntegrationWithOtherOptions ensures the new timeout functions can be combined with existing options
func TestIntegrationWithOtherOptions(t *testing.T) {
	workflowTimeout := 30 * time.Second
	activityTimeout := 45 * time.Second
	stickyTimeout := 10 * time.Second
	maxHistorySize := int64(5000)

	opts := ApplyOptions(
		WithWorkflowLockTimeout(workflowTimeout),
		WithActivityLockTimeout(activityTimeout),
		WithStickyTimeout(stickyTimeout),
		WithMaxHistorySize(maxHistorySize),
	)

	require.Equal(t, workflowTimeout, opts.WorkflowLockTimeout)
	require.Equal(t, activityTimeout, opts.ActivityLockTimeout)
	require.Equal(t, stickyTimeout, opts.StickyTimeout)
	require.Equal(t, maxHistorySize, opts.MaxHistorySize)
}
