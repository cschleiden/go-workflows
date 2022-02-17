package workflow

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/internal/fn"
	"github.com/cschleiden/go-workflows/internal/sync"
	"github.com/stretchr/testify/require"
)

func reg_workflow1(ctx sync.Context) error {
	return nil
}

func Test_WorkflowRegistration(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	err := r.RegisterWorkflow(reg_workflow1)
	require.NoError(t, err)

	x, err := r.GetWorkflow("reg_workflow1")
	require.NoError(t, err)

	fn, ok := x.(func(context sync.Context) error)
	require.True(t, ok)
	require.NotNil(t, fn)

	err = fn(sync.Background())
	require.NoError(t, err)
}

func reg_activity(ctx context.Context) error {
	return nil
}

func Test_ActivityRegistration(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	err := r.RegisterActivity(reg_activity)
	require.NoError(t, err)

	x, err := r.GetActivity("reg_activity")
	require.NoError(t, err)

	fn, ok := x.(func(context context.Context) error)
	require.True(t, ok)
	require.NotNil(t, fn)

	err = fn(context.Background())
	require.NoError(t, err)
}

type reg_activities struct {
	SomeValue string
}

func (r *reg_activities) Activity1(ctx context.Context) (string, error) {
	return r.SomeValue, nil
}

func (r *reg_activities) privateActivity(ctx context.Context) error {
	return nil
}

func Test_ActivityRegistrationOnStruct(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	a := &reg_activities{
		SomeValue: "test",
	}
	err := r.RegisterActivity(a)
	require.NoError(t, err)

	b := &reg_activities{}
	x, err := r.GetActivity(fn.Name(b.Activity1))
	require.NoError(t, err)

	// Ignore private methods
	y, err := r.GetActivity(fn.Name(b.privateActivity))
	require.Error(t, err)
	require.Nil(t, y)

	fn, ok := x.(func(context context.Context) (string, error))
	require.True(t, ok)
	require.NotNil(t, fn)

	v, err := fn(context.Background())
	require.NoError(t, err)
	require.Equal(t, "test", v)
}
