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

	autoExpirationTime := time.Second * 2

	redisClient := getClient()
	setup := getCreateBackend(redisClient, WithAutoExpiration(autoExpirationTime))
	b := setup()

	c := client.New(b)
	w := worker.New(b, nil)

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, w.Start(ctx))

	wf := func(ctx workflow.Context) error {
		return nil
	}

	require.NoError(t, w.RegisterWorkflow(wf))

	wfi, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, wf)
	require.NoError(t, err)

	require.NoError(t, c.WaitForWorkflowInstance(ctx, wfi, time.Second*10))

	// Wait for redis to expire the keys
	time.Sleep(autoExpirationTime * 2)

	_, err = b.GetWorkflowInstanceState(ctx, wfi)
	require.ErrorIs(t, err, backend.ErrInstanceNotFound)

	// Check that the instance is gone from the list of instances
	insts, err := b.(*redisBackend).GetWorkflowInstances(ctx, "", "", 1)
	require.NoError(t, err)
	require.Empty(t, insts)

	cancel()
	require.NoError(t, w.WaitForCompletion())
}

func Test_AutoExpiration_SubWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	autoExpirationTime := time.Second * 2

	redisClient := getClient()
	setup := getCreateBackend(redisClient, WithAutoExpiration(autoExpirationTime))
	b := setup()

	c := client.New(b)
	w := worker.New(b, nil)

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, w.Start(ctx))
	defer func() {
		cancel()

		require.NoError(t, w.WaitForCompletion())
	}()

	swf := func(ctx workflow.Context) (int, error) {
		return 42, nil
	}

	swfInstanceID := uuid.NewString()

	wf := func(ctx workflow.Context) (int, error) {
		l := workflow.Logger(ctx)
		l.Debug("Starting sub workflow", "instanceID", swfInstanceID)

		r, err := workflow.CreateSubWorkflowInstance[int](ctx, workflow.SubWorkflowOptions{
			InstanceID: swfInstanceID,
		}, swf).Get(ctx)

		if _, err := workflow.ScheduleTimer(ctx, time.Second*2).Get(ctx); err != nil {
			return 0, err
		}

		return r, err
	}

	require.NoError(t, w.RegisterWorkflow(wf))
	require.NoError(t, w.RegisterWorkflow(swf))

	wfi, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, wf)
	require.NoError(t, err)

	r, err := client.GetWorkflowResult[int](ctx, c, wfi, time.Second*10)
	require.NoError(t, err)
	require.Equal(t, 42, r)

	// Wait for redis to expire the keys
	time.Sleep(autoExpirationTime * 2)

	// Main workflow should now be expired
	_, err = b.GetWorkflowInstanceState(ctx, wfi)
	require.ErrorIs(t, err, backend.ErrInstanceNotFound)
}

func Test_AutoExpiration_ContinueAsNew_SubWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	autoExpirationTime := time.Second * 2

	redisClient := getClient()
	setup := getCreateBackend(redisClient, WithAutoExpiration(0), WithAutoExpirationContinueAsNew(autoExpirationTime))
	b := setup()

	c := client.New(b)
	w := worker.New(b, nil)

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, w.Start(ctx))
	defer func() {
		cancel()

		require.NoError(t, w.WaitForCompletion())
	}()

	var swfInstances []*workflow.Instance

	swf := func(ctx workflow.Context, iteration int) (int, error) {
		if iteration > 3 {
			return 42, nil
		}

		// Keep track of continuedasnew instances
		swfInstances = append(swfInstances, workflow.WorkflowInstance(ctx))

		return 0, workflow.ContinueAsNew(ctx, iteration+1)
	}

	swfInstanceID := uuid.NewString()

	wf := func(ctx workflow.Context) (int, error) {
		l := workflow.Logger(ctx)
		l.Debug("Starting sub workflow", "instanceID", swfInstanceID)

		r, err := workflow.CreateSubWorkflowInstance[int](ctx, workflow.SubWorkflowOptions{
			InstanceID: swfInstanceID,
		}, swf, 0).Get(ctx)

		if _, err := workflow.ScheduleTimer(ctx, time.Second*2).Get(ctx); err != nil {
			return 0, err
		}

		return r, err
	}

	require.NoError(t, w.RegisterWorkflow(wf))
	require.NoError(t, w.RegisterWorkflow(swf))

	wfi, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, wf)
	require.NoError(t, err)

	// Wait for redis to expire the keys
	time.Sleep(autoExpirationTime * 2)

	// Main workflow should still be there
	r, err := client.GetWorkflowResult[int](ctx, c, wfi, time.Second*10)
	require.NoError(t, err)
	require.Equal(t, 42, r)

	// All continued-as-new sub-workflow instances should be expired
	for _, swfInstance := range swfInstances {
		_, err = b.GetWorkflowInstanceState(ctx, swfInstance)
		require.ErrorIs(t, err, backend.ErrInstanceNotFound)
	}
}
