package workflow

import (
	"testing"

	"github.com/cschleiden/go-dt/internal/sync"
	"github.com/stretchr/testify/require"
)

func reg_workflow1(ctx sync.Context) error {
	return nil
}

func Test_WorkflowRegistration(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	err := r.RegisterWorkflow(reg_workflow1)
	require.NoError(t, err)

	x, err := r.GetWorkflow("reg_workflow1")
	require.NoError(t, err)

	fn, ok := x.(func(context sync.Context) error)
	require.True(t, ok)
	require.NotNil(t, fn)

	err = fn(sync.Background())
	require.NoError(t, err)
}
