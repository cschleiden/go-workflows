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
		require.NoError(t, b.Close())
	})

	bd := b.(diag.Backend)

	ctx := context.Background()
	instances, err := bd.GetWorkflowInstances(ctx, "", "", 5)
	require.NoError(t, err)
	require.Empty(t, instances)

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

func Test_Diag_GetWorkflowInstances_Ordering(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	rclient := getClient()
	setup := getCreateBackend(rclient)

	b := setup()

	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	bd := b.(diag.Backend)
	c := client.New(b)
	ctx := context.Background()

	for _, id := range []string{"inst-a", "inst-b", "inst-c"} {
		_, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{InstanceID: id}, "some-workflow")
		require.NoError(t, err)
	}

	// Results should be newest-first (Rev: true).
	instances, err := bd.GetWorkflowInstances(ctx, "", "", 10)
	require.NoError(t, err)
	require.Len(t, instances, 3)
	require.Equal(t, "inst-c", instances[0].Instance.InstanceID)
	require.Equal(t, "inst-a", instances[2].Instance.InstanceID)
}

func Test_Diag_GetWorkflowInstances_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	rclient := getClient()
	setup := getCreateBackend(rclient)

	b := setup()

	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	bd := b.(diag.Backend)
	c := client.New(b)
	ctx := context.Background()

	for _, id := range []string{"inst-1", "inst-2", "inst-3", "inst-4"} {
		_, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{InstanceID: id}, "some-workflow")
		require.NoError(t, err)
	}

	// First page: newest 2.
	page1, err := bd.GetWorkflowInstances(ctx, "", "", 2)
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// Second page: continue after last item of page 1.
	last := page1[len(page1)-1]
	page2, err := bd.GetWorkflowInstances(ctx, last.Instance.InstanceID, last.Instance.ExecutionID, 2)
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// No overlap between pages.
	page1IDs := map[string]bool{page1[0].Instance.InstanceID: true, page1[1].Instance.InstanceID: true}
	for _, inst := range page2 {
		require.False(t, page1IDs[inst.Instance.InstanceID], "duplicate instance across pages: %s", inst.Instance.InstanceID)
	}
}
