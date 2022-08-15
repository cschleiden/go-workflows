package command

import (
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/stretchr/testify/require"
)

func assertExecuteNoEvent(t *testing.T, c Command, expectedState CommandState) {
	r := c.Execute(clock.New())

	require.Nil(t, r)
}

func assertExecuteWithEvent(t *testing.T, c Command, expectedState CommandState, expectedEventType history.EventType) *CommandResult {
	r := c.Execute(clock.New())

	require.NotNil(t, r)
	require.Equal(t, expectedState, c.State())
	require.Len(t, r.Events, 1)
	require.Equal(t, expectedEventType, r.Events[0].Type)

	return r
}
