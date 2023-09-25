package internal

import (
	"context"
	"math/rand"

	"github.com/cschleiden/go-workflows/workflow"
)

type MidInput struct {
	FanOut     int
	LeafFanOut int
	Depth      int

	Activities       int
	PayloadSizeBytes int
}

func waitAll[T any](ctx workflow.Context, futures []workflow.Future[T]) error {
	for _, f := range futures {
		_, err := f.Get(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func Root(ctx workflow.Context, input *MidInput) error {
	f := make([]workflow.Future[any], 0)

	for i := 0; i < input.FanOut; i++ {
		f = append(f, workflow.CreateSubWorkflowInstance[any](ctx, workflow.DefaultSubWorkflowOptions, Mid, &MidInput{
			FanOut: input.FanOut,
			Depth:  input.Depth - 1,

			LeafFanOut: input.LeafFanOut,

			Activities:       input.Activities,
			PayloadSizeBytes: input.PayloadSizeBytes,
		}))
	}

	return waitAll(ctx, f)
}

func Mid(ctx workflow.Context, input *MidInput) error {
	f := make([]workflow.Future[any], 0)

	if input.Depth == 0 {
		for i := 0; i < input.LeafFanOut; i++ {
			f = append(f, workflow.CreateSubWorkflowInstance[any](ctx, workflow.DefaultSubWorkflowOptions, Leaf, &LeafInput{
				Activities:       input.Activities,
				PayloadSizeBytes: input.PayloadSizeBytes,
			}))
		}
	} else {
		for i := 0; i < input.FanOut; i++ {
			f = append(f, workflow.CreateSubWorkflowInstance[any](ctx, workflow.DefaultSubWorkflowOptions, Mid, &MidInput{
				FanOut: input.FanOut,
				Depth:  input.Depth - 1,

				LeafFanOut: input.LeafFanOut,

				Activities:       input.Activities,
				PayloadSizeBytes: input.PayloadSizeBytes,
			}))
		}
	}

	return waitAll(ctx, f)
}

type LeafInput struct {
	Activities       int
	PayloadSizeBytes int
}

func Leaf(ctx workflow.Context, input *LeafInput) error {
	f := make([]workflow.Future[string], 0)

	for i := 0; i < input.Activities; i++ {
		f = append(f, workflow.ExecuteActivity[string](ctx, workflow.DefaultActivityOptions, Activity, &activityInput{
			PayloadSizeBytes: input.PayloadSizeBytes,
		}))
	}

	return waitAll(ctx, f)
}

type activityInput struct {
	PayloadSizeBytes int
}

func Activity(ctx context.Context, input *activityInput) (string, error) {
	// Generate payload
	res := randSeq(input.PayloadSizeBytes)

	return res, nil
}

var alphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randSeq(n int) string {
	rand.Seed(42)

	b := make([]rune, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}
