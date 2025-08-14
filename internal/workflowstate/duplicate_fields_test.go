package workflowstate

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestLogFieldDuplication(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	
	// Create JSON logger to easily see field duplication
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create an instance 
	instance := &core.WorkflowInstance{
		InstanceID:  "test-instance-123",
		ExecutionID: "test-execution-456",
	}

	// This mimics what happens in executor.go NewExecutor
	logger = logger.With(
		slog.String(log.InstanceIDKey, instance.InstanceID),
		slog.String(log.ExecutionIDKey, instance.ExecutionID),
	)

	tracer := noop.NewTracerProvider().Tracer("test")
	clock := clock.New()
	state := NewWorkflowState(instance, logger, tracer, clock)

	// Log something using the workflow state logger (which currently duplicates fields)
	state.Logger().Debug("Test message from workflow state")
	
	// Parse and analyze the log output
	logLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	
	foundDuplicates := false
	for _, line := range logLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		t.Logf("Log entry: %s", line)
		
		// Count field occurrences in raw JSON string instead of unmarshaling
		// since JSON unmarshaling overwrites duplicate keys
		instanceCount := strings.Count(line, `"`+log.InstanceIDKey+`"`)
		executionCount := strings.Count(line, `"`+log.ExecutionIDKey+`"`)
		
		t.Logf("Instance ID field count: %d", instanceCount)
		t.Logf("Execution ID field count: %d", executionCount)
		
		if instanceCount > 1 || executionCount > 1 {
			foundDuplicates = true
			t.Logf("❌ DUPLICATE FIELDS DETECTED!")
		} else {
			t.Logf("✅ No duplicates found")
		}
	}
	
	// After fix, we should NOT find duplicates
	assert.False(t, foundDuplicates, "Should not find duplicate fields after fix")
}