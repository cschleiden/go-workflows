package executor

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/internal/contextvalue"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
)

func Test_Workflow_WrapsPanic(t *testing.T) {
	f := func() {
		panic("wf panic")
	}

	w := func(ctx sync.Context) error {
		f()

		return nil
	}

	ctx := sync.Background()
	ctx = contextvalue.WithConverter(ctx, converter.DefaultConverter)

	wf := newWorkflow(reflect.ValueOf(w))
	err := wf.Execute(ctx, nil)
	require.NoError(t, err)

	for !wf.Completed() {
		require.NoError(t, wf.Continue())
	}

	wfErr := wf.Error()
	require.Error(t, wfErr)
	var panicErr *workflowerrors.PanicError
	require.ErrorAs(t, wfErr, &panicErr)
}
