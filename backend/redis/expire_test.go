package redis

import (
	"context"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_AutoExpiration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	autoExpirationTime := time.Second * 1

	redisClient := getClient()
	setup := getCreateBackend(redisClient, WithAutoExpiration(autoExpirationTime))
	b := setup()

	c := client.New(b)
	w := worker.New(b, &worker.DefaultWorkerOptions)

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, w.Start(ctx))

	wf := func(ctx workflow.Context) error {
		return nil
	}

	wfi, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, wf)
	require.NoError(t, err)

	require.NoError(t, c.WaitForWorkflowInstance(ctx, wfi, time.Second*10))

	// Wait for redis to expire the keys
	time.Sleep(autoExpirationTime)

	_, err = b.GetWorkflowInstanceState(ctx, wfi)
	require.ErrorIs(t, err, backend.ErrInstanceNotFound)

	cancel()
	require.NoError(t, w.WaitForCompletion())
}
