package tester

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/stretchr/testify/require"
)

type myKey int

var k myKey

type myData struct {
	Foo string
	Bar int
}

func withMyValues(ctx context.Context, d *myData) context.Context {
	return context.WithValue(ctx, k, d)
}

func myValues(ctx context.Context) *myData {
	return ctx.Value(k).(*myData)
}

func withMyValuesWf(ctx workflow.Context, d *myData) workflow.Context {
	return workflow.WithValue(ctx, k, d)
}

func myValuesWf(ctx workflow.Context) *myData {
	return ctx.Value(k).(*myData)
}

type myPropagator struct {
}

var _ workflow.ContextPropagator = &myPropagator{}

// Extract implements workflow.ContextPropagator
func (*myPropagator) Extract(ctx context.Context, metadata *metadata.WorkflowMetadata) (context.Context, error) {
	ms := metadata.Get("mydata")

	var d *myData
	if err := json.Unmarshal([]byte(ms), &d); err != nil {
		return nil, err
	}

	return withMyValues(ctx, d), nil
}

// ExtractToWorkflow implements workflow.ContextPropagator
func (*myPropagator) ExtractToWorkflow(ctx sync.Context, metadata *metadata.WorkflowMetadata) (sync.Context, error) {
	ms := metadata.Get("mydata")

	var d *myData
	if err := json.Unmarshal([]byte(ms), &d); err != nil {
		return nil, err
	}

	return withMyValuesWf(ctx, d), nil
}

// Inject implements workflow.ContextPropagator
func (*myPropagator) Inject(ctx context.Context, metadata *metadata.WorkflowMetadata) error {
	d := myValues(ctx)
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	metadata.Set("mydata", string(b))

	return nil
}

// InjectFromWorkflow implements workflow.ContextPropagator
func (*myPropagator) InjectFromWorkflow(ctx sync.Context, metadata *metadata.WorkflowMetadata) error {
	d := myValuesWf(ctx)
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	metadata.Set("mydata", string(b))

	return nil
}

func Test_ContextPropagation(t *testing.T) {
	activity1 := func(ctx context.Context) (string, error) {
		d := myValues(ctx)

		return fmt.Sprintf("%s%d", d.Foo, d.Bar), nil
	}

	wf := func(ctx workflow.Context) (string, error) {
		ar, err := workflow.ExecuteActivity[string](
			ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)
		if err != nil {
			return "", err
		}
		d := myValuesWf(ctx)
		return ar + d.Foo, nil
	}

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*3), WithContextPropagator(&myPropagator{}))

	require.NoError(t, tester.registry.RegisterActivity(activity1))

	ctx := context.Background()
	ctx = withMyValues(ctx, &myData{Foo: "foo", Bar: 42})

	tester.Execute(ctx)

	require.True(t, tester.WorkflowFinished())
	wr, _ := tester.WorkflowResult()
	require.Equal(t, "foo42foo", wr)

	tester.AssertExpectations(t)
}

func Test_ContextPropagation_Subworkflow(t *testing.T) {
	activity1 := func(ctx context.Context) (string, error) {
		d := myValues(ctx)
		return fmt.Sprintf("%s%d", d.Foo, d.Bar), nil
	}

	swf := func(ctx workflow.Context) (string, error) {
		d := myValuesWf(ctx)
		return fmt.Sprintf("%s%d", d.Foo, d.Bar), nil
	}

	wf := func(ctx workflow.Context) (string, error) {
		ar, err := workflow.ExecuteActivity[string](
			ctx, workflow.DefaultActivityOptions, activity1).Get(ctx)
		if err != nil {
			return "", err
		}

		swfr, err := workflow.CreateSubWorkflowInstance[string](
			ctx, workflow.DefaultSubWorkflowOptions, swf).Get(ctx)
		if err != nil {
			return "", err
		}

		d := myValuesWf(ctx)
		return ar + swfr + d.Foo, nil
	}

	tester := NewWorkflowTester[string](wf, WithTestTimeout(time.Second*3), WithContextPropagator(&myPropagator{}))

	require.NoError(t, tester.registry.RegisterActivity(activity1))
	require.NoError(t, tester.registry.RegisterWorkflow(swf))

	ctx := context.Background()
	ctx = withMyValues(ctx, &myData{Foo: "foo", Bar: 42})

	tester.Execute(ctx)

	require.True(t, tester.WorkflowFinished())
	wr, werr := tester.WorkflowResult()
	require.NoError(t, werr)
	require.Equal(t, "foo42foo42foo", wr)

	tester.AssertExpectations(t)
}
