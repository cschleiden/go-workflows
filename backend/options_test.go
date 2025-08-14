package backend

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithWorkflowLockTimeout(t *testing.T) {
	timeout := 5 * time.Minute
	option := WithWorkflowLockTimeout(timeout)

	opts := ApplyOptions(option)

	assert.Equal(t, timeout, opts.WorkflowLockTimeout)
}

func TestWithActivityLockTimeout(t *testing.T) {
	timeout := 3 * time.Minute
	option := WithActivityLockTimeout(timeout)

	opts := ApplyOptions(option)

	assert.Equal(t, timeout, opts.ActivityLockTimeout)
}

func TestWithWorkflowAndActivityLockTimeout(t *testing.T) {
	workflowTimeout := 2 * time.Minute
	activityTimeout := 4 * time.Minute

	opts := ApplyOptions(
		WithWorkflowLockTimeout(workflowTimeout),
		WithActivityLockTimeout(activityTimeout),
	)

	assert.Equal(t, workflowTimeout, opts.WorkflowLockTimeout)
	assert.Equal(t, activityTimeout, opts.ActivityLockTimeout)
}

func TestDefaultValues(t *testing.T) {
	opts := ApplyOptions()

	// Verify default values are preserved when no options are provided
	assert.Equal(t, time.Minute, opts.WorkflowLockTimeout)
	assert.Equal(t, time.Minute*2, opts.ActivityLockTimeout)
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

	assert.Equal(t, workflowTimeout, opts.WorkflowLockTimeout)
	assert.Equal(t, activityTimeout, opts.ActivityLockTimeout)
	assert.Equal(t, stickyTimeout, opts.StickyTimeout)
	assert.Equal(t, maxHistorySize, opts.MaxHistorySize)
}
