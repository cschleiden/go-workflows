package main

import (
	"context"
	"encoding/json"

	"github.com/cschleiden/go-workflows/workflow"
)

type myKey int

var k myKey

type myData struct {
	Name  string
	Count int
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

type myPropagator struct{}

var _ workflow.ContextPropagator = &myPropagator{}

// Extract implements workflow.ContextPropagator
func (*myPropagator) Extract(ctx context.Context, metadata *workflow.Metadata) (context.Context, error) {
	ms := metadata.Get("mydata")

	var d *myData
	if err := json.Unmarshal([]byte(ms), &d); err != nil {
		return nil, err
	}

	return withMyValues(ctx, d), nil
}

// ExtractToWorkflow implements workflow.ContextPropagator
func (*myPropagator) ExtractToWorkflow(ctx workflow.Context, metadata *workflow.Metadata) (workflow.Context, error) {
	ms := metadata.Get("mydata")

	var d *myData
	if err := json.Unmarshal([]byte(ms), &d); err != nil {
		return nil, err
	}

	return withMyValuesWf(ctx, d), nil
}

// Inject implements workflow.ContextPropagator
func (*myPropagator) Inject(ctx context.Context, metadata *workflow.Metadata) error {
	d := myValues(ctx)
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	metadata.Set("mydata", string(b))

	return nil
}

// InjectFromWorkflow implements workflow.ContextPropagator
func (*myPropagator) InjectFromWorkflow(ctx workflow.Context, metadata *workflow.Metadata) error {
	d := myValuesWf(ctx)
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}

	metadata.Set("mydata", string(b))

	return nil
}
