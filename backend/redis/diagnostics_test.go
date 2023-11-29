package redis

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/stretchr/testify/require"
)

func Test_Diag_GetWorkflowInstances(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	rclient := getClient()
	setup := getCreateBackend(rclient)

	b := setup()

	t.Cleanup(func() {
		b.Close()
	})

	bd := b.(diag.Backend)

	ctx := context.Background()
	instances, err := bd.GetWorkflowInstances(ctx, "", "", 5)
	require.NoError(t, err)
	require.Len(t, instances, 0)

	c := client.New(b)

	_, err = c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: "ex1",
	}, "some-workflow")
	require.NoError(t, err)

	instances, err = bd.GetWorkflowInstances(ctx, "", "", 5)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	require.Equal(t, "ex1", instances[0].Instance.InstanceID)
}
