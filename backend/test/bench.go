package test

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const PayloadSizeBytes = 1 * 1024 * 1024 // 1 MB

var alphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randSeq(n int) string {
	rand.Seed(42)

	b := make([]rune, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}

var data = randSeq(PayloadSizeBytes)

func SimpleWorkflowBenchmark(b *testing.B, setup func() TestBackend, teardown func(b TestBackend)) {
	// Suppress default metric
	b.ReportMetric(0, "ns/op")
	b.StopTimer()

	backend := setup()

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

	log.Println("Starting", toStart, "workflows...")

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

	if teardown != nil {
		teardown(backend)
	}
}

func startAndCompleteWorkflow(ctx context.Context, b *testing.B, c client.Client, wg *sync.WaitGroup) {
	defer wg.Done()

	wf, err := c.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: uuid.NewString(),
	}, simpleWorkflow, &simpleWorkflowInput{
		In1:  1,
		Data: data,
	})
	require.NoError(b, err)

	r, err := client.GetWorkflowResult[*simpleWorkflowResult](ctx, c, wf, time.Second*30)
	require.NoError(b, err)
	require.Equal(b, 4, r.Result)
}

type simpleWorkflowInput struct {
	In1  int
	Data string
}

type simpleWorkflowResult struct {
	Result int
	Data   string
}

func simpleWorkflow(ctx workflow.Context, input *simpleWorkflowInput) (*simpleWorkflowResult, error) {
	r1, err := workflow.ExecuteActivity[int](ctx, workflow.DefaultActivityOptions, simpleActivity1).Get(ctx)
	if err != nil {
		return nil, err
	}

	var r2r int
	for i := 0; i < 2; i++ {
		r2, err := workflow.ExecuteActivity[*simpleActivity2result](ctx, workflow.DefaultActivityOptions, simpleActivity2).Get(ctx)
		if err != nil {
			return nil, err
		}

		r2r = r2.Result
	}

	data, _ := workflow.SideEffect(ctx, func(ctx workflow.Context) string {
		return data
	}).Get(ctx)

	return &simpleWorkflowResult{
		Result: input.In1 + r1 + r2r,
		Data:   data,
	}, nil
}

func simpleActivity1(ctx context.Context) (int, error) {
	return 1, nil
}

type simpleActivity2result struct {
	Result int
	Data   string
}

func simpleActivity2(ctx context.Context) (*simpleActivity2result, error) {
	return &simpleActivity2result{
		Result: 2,
		Data:   data,
	}, nil
}
