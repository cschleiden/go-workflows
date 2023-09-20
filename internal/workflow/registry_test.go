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

func TestRegistry_RegisterWorkflow(t *testing.T) {
	type args struct {
		name     string
		workflow Workflow
	}
	tests := []struct {
		name     string
		args     args
		wantName string
		wantErr  bool
	}{
		{
			name: "valid workflow",
			args: args{
				workflow: reg_workflow1,
			},
			wantName: "github.com/cschleiden/go-workflows/internal/workflow.reg_workflow1",
		},
		{
			name: "valid workflow by name",
			args: args{
				name:     "CustomName",
				workflow: reg_workflow1,
			},
			wantName: "CustomName",
		},
		{
			name: "valid workflow with results",
			args: args{
				workflow: func(ctx sync.Context) (int, error) { return 42, nil },
			},
		},
		{
			name: "valid workflow with multiple parameters",
			args: args{
				workflow: func(ctx sync.Context, a, b int) (int, error) { return 42, nil },
			},
		},
		{
			name: "missing parameter",
			args: args{
				workflow: func(ctx context.Context) {},
			},
			wantErr: true,
		},
		{
			name: "missing error result",
			args: args{
				workflow: func(ctx sync.Context) {},
			},
			wantErr: true,
		},
		{
			name: "missing error with results",
			args: args{
				workflow: func(ctx sync.Context) int { return 42 },
			},
			wantErr: true,
		},
		{
			name: "missing error with results",
			args: args{
				workflow: func(ctx sync.Context) int { return 42 },
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()

			cfg := RegisterConfig{Name: tt.args.name}
			err := r.RegisterWorkflow(tt.args.workflow, &cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("Registry.RegisterWorkflow() error = %v, wantErr %v", err, tt.wantErr)
				t.FailNow()
			}

			if tt.wantName != "" {
				x, err := r.GetWorkflow(tt.wantName)
				require.NoError(t, err)
				require.NotNil(t, x)
			}
		})
	}
}

func Test_RegisterWorkflow_Conflict(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	var wantErr *ErrWorkflowAlreadyRegistered

	err := r.RegisterWorkflow(reg_workflow1, nil)
	require.NoError(t, err)

	err = r.RegisterWorkflow(reg_workflow1, nil)
	require.ErrorAs(t, err, &wantErr)

	err = r.RegisterWorkflow(reg_workflow1, &RegisterConfig{Name: "CustomName"})
	require.NoError(t, err)

	err = r.RegisterWorkflow(reg_workflow1, &RegisterConfig{Name: "CustomName"})
	require.ErrorAs(t, err, &wantErr)
}

func reg_activity(ctx context.Context) error {
	return nil
}

func Test_ActivityRegistration(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	err := r.RegisterActivity(reg_activity, nil)
	require.NoError(t, err)

	x, err := r.GetActivity("github.com/cschleiden/go-workflows/internal/workflow.reg_activity")
	require.NoError(t, err)

	fn, ok := x.(func(context context.Context) error)
	require.True(t, ok)
	require.NotNil(t, fn)

	err = fn(context.Background())
	require.NoError(t, err)

	err = r.RegisterActivity(reg_activity, &RegisterConfig{Name: "CustomName"})
	require.NoError(t, err)

	x, err = r.GetActivity("CustomName")
	require.NoError(t, err)
}

func reg_activity_invalid(ctx context.Context) {
}

func Test_ActivityRegistration_Invalid(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	err := r.RegisterActivity(reg_activity_invalid, nil)
	require.Error(t, err)
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
	err := r.RegisterActivity(a, nil)
	require.NoError(t, err)

	b := &reg_activities{}
	x, err := r.GetActivity(fn.FuncName(b.Activity1))
	require.NoError(t, err)

	// Ignore private methods
	y, err := r.GetActivity(fn.FuncName(b.privateActivity))
	require.Error(t, err)
	require.Nil(t, y)

	fn, ok := x.(func(context context.Context) (string, error))
	require.True(t, ok)
	require.NotNil(t, fn)

	v, err := fn(context.Background())
	require.NoError(t, err)
	require.Equal(t, "test", v)
}

func Test_RegisterActivity_Conflict(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	var wantErr *ErrActivityAlreadyRegistered

	err := r.RegisterActivity(reg_activity, nil)
	require.NoError(t, err)

	err = r.RegisterActivity(reg_activity, nil)
	require.ErrorAs(t, err, &wantErr)

	err = r.RegisterActivity(reg_activity, &RegisterConfig{Name: "CustomName"})
	require.NoError(t, err)

	err = r.RegisterActivity(reg_activity, &RegisterConfig{Name: "CustomName"})
	require.ErrorAs(t, err, &wantErr)
}

type reg_invalid_activities struct {
	SomeValue string
}

func (r *reg_invalid_activities) Activity1(ctx context.Context) {
}

func Test_ActivityRegistrationOnStruct_Invalid(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	a := &reg_invalid_activities{
		SomeValue: "test",
	}
	err := r.RegisterActivity(a, nil)
	require.Error(t, err)
}
