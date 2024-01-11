package workflowstate

import (
	"log/slog"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_PendingFutures(t *testing.T) {
	i := core.NewWorkflowInstance(uuid.NewString(), "")

	wfState := NewWorkflowState(i, slog.Default(), clock.New())

	require.False(t, wfState.HasPendingFutures())

	f := sync.NewFuture[int]()
	wfState.TrackFuture(1, AsDecodingSettable[int](converter.DefaultConverter, "f", f))

	require.True(t, wfState.HasPendingFutures())

	wfState.RemoveFuture(1)

	require.False(t, wfState.HasPendingFutures())
}
