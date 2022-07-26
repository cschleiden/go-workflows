package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/ticctech/go-workflows/client"
	"github.com/ticctech/go-workflows/worker"
	"github.com/ticctech/go-workflows/workflow"
)

func SimpleWorkflowBenchmark(b *testing.B, setup func() TestBackend, teardown func(b TestBackend)) {
	// Suppress default metric
	b.ReportMetric(0, "ns/op")
	b.StopTimer()

	backend := setup()
	if teardown != nil {
		defer teardown(backend)
	}

	w := worker.New(backend, &worker.Options{
		WorkflowPollers: 1,
		ActivityPollers: 1,
	})
	c := client.New(backend)

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(b, w.Start(ctx))

	require.NoError(b, w.RegisterWorkflow(simpleWorkflow))
	require.NoError(b, w.RegisterActivity(simpleActivity1))
	require.NoError(b, w.RegisterActivity(simpleActivity2))

	b.StartTimer()

	toStart := 100

	wg := &sync.WaitGroup{}
	for i := 0; i < toStart; i++ {
		wg.Add(1)
		go startAndCompleteWorkflow(ctx, b, c, wg)
	}
	wg.Wait()

	b.StopTimer()
	b.ReportMetric(float64(toStart), "Workflows")

	cancel()
	require.NoError(b, w.WaitForCompletion())
}

func startAndCompleteWorkflow(ctx context.Context, b *testing.B, c client.Client, wg *sync.WaitGroup) {
	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, simpleWorkflow, 2)
	require.NoError(b, err)

	r, err := client.GetWorkflowResult[int](ctx, c, wf, time.Second*30)
	require.NoError(b, err)
	require.Equal(b, 5, r)

	wg.Done()
}

func simpleWorkflow(ctx workflow.Context, in1 int) (int, error) {
	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, simpleActivity1).Get(ctx)
	if err != nil {
		return 0, err
	}

	r2, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, simpleActivity2).Get(ctx)
	if err != nil {
		return 0, err
	}

	return in1 + r1 + r2, nil
}

func simpleActivity1(ctx context.Context) (int, error) {
	return 1, nil
}

func simpleActivity2(ctx context.Context) (int, error) {
	return 2, nil
}
