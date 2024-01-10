package workflowstate

import (
	"log/slog"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_ReplayLogger_With(t *testing.T) {
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	wfState := NewWorkflowState(i, slog.Default(), clock.New())

	with := wfState.Logger().With(slog.String("foo", "bar"))
	require.IsType(t, &replayHandler{}, with.Handler())
}

func Test_ReplayLogger_WithGroup(t *testing.T) {
	i := core.NewWorkflowInstance(uuid.NewString(), "")
	wfState := NewWorkflowState(i, slog.Default(), clock.New())

	with := wfState.Logger().WithGroup("group_name")
	require.IsType(t, &replayHandler{}, with.Handler())
}
