package registry

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
		workflow any
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
			wantName: "reg_workflow1",
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
			r := New()

			err := r.RegisterWorkflow(tt.args.workflow, WithName(tt.args.name))

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
	r := New()
	require.NotNil(t, r)

	var wantErr *ErrWorkflowAlreadyRegistered

	err := r.RegisterWorkflow(reg_workflow1)
	require.NoError(t, err)

	err = r.RegisterWorkflow(reg_workflow1)
	require.ErrorAs(t, err, &wantErr)

	err = r.RegisterWorkflow(reg_workflow1, WithName("CustomName"))
	require.NoError(t, err)

	err = r.RegisterWorkflow(reg_workflow1, WithName("CustomName"))
	require.ErrorAs(t, err, &wantErr)
}

func reg_activity(ctx context.Context) error {
	return nil
}

func Test_ActivityRegistration(t *testing.T) {
	r := New()
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

	err = r.RegisterActivity(reg_activity, WithName("CustomName"))
	require.NoError(t, err)

	_, err = r.GetActivity("CustomName")
	require.NoError(t, err)
}

func reg_activity_invalid(ctx context.Context) {
}

func Test_ActivityRegistration_Invalid(t *testing.T) {
	r := New()
	require.NotNil(t, r)

	err := r.RegisterActivity(reg_activity_invalid)
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
	r := New()
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

func Test_RegisterActivity_Conflict(t *testing.T) {
	r := New()
	require.NotNil(t, r)

	var wantErr *ErrActivityAlreadyRegistered

	err := r.RegisterActivity(reg_activity)
	require.NoError(t, err)

	err = r.RegisterActivity(reg_activity)
	require.ErrorAs(t, err, &wantErr)

	err = r.RegisterActivity(reg_activity, WithName("CustomName"))
	require.NoError(t, err)

	err = r.RegisterActivity(reg_activity, WithName("CustomName"))
	require.ErrorAs(t, err, &wantErr)
}

type reg_invalid_activities struct {
	SomeValue string
}

func (r *reg_invalid_activities) Activity1(ctx context.Context) {
}

func Test_ActivityRegistrationOnStruct_Invalid(t *testing.T) {
	r := New()
	require.NotNil(t, r)

	a := &reg_invalid_activities{
		SomeValue: "test",
	}
	err := r.RegisterActivity(a)
	require.Error(t, err)
}
func reg_workflow_v1(ctx sync.Context) error {
	return nil
}

func reg_workflow_v2(ctx sync.Context) (string, error) {
	return "v2", nil
}

func TestRegistry_RegisterVersionedWorkflow(t *testing.T) {
	tests := []struct {
		name        string
		workflow    any
		version     string
		workflowName string
		wantErr     bool
	}{
		{
			name:     "valid versioned workflow",
			workflow: reg_workflow_v1,
			version:  "1.0.0",
		},
		{
			name:         "valid versioned workflow with custom name",
			workflow:     reg_workflow_v1,
			version:      "1.0.0",
			workflowName: "CustomWorkflow",
		},
		{
			name:     "valid versioned workflow with results",
			workflow: reg_workflow_v2,
			version:  "2.0.0",
		},
		{
			name:     "empty version",
			workflow: reg_workflow_v1,
			version:  "",
			wantErr:  true,
		},
		{
			name:     "invalid workflow - not a function",
			workflow: "not a function",
			version:  "1.0.0",
			wantErr:  true,
		},
		{
			name:     "invalid workflow - missing context",
			workflow: func() error { return nil },
			version:  "1.0.0",
			wantErr:  true,
		},
		{
			name:     "invalid workflow - missing error return",
			workflow: func(ctx sync.Context) {},
			version:  "1.0.0",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()

			var opts []RegisterOption
			if tt.workflowName != "" {
				opts = append(opts, WithName(tt.workflowName))
			}

			err := r.RegisterVersionedWorkflow(tt.workflow, tt.version, opts...)

			if (err != nil) != tt.wantErr {
				t.Errorf("Registry.RegisterVersionedWorkflow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				expectedName := tt.workflowName
				if expectedName == "" {
					expectedName = fn.Name(tt.workflow)
				}

				workflow, err := r.GetVersionedWorkflow(expectedName, tt.version)
				require.NoError(t, err)
				require.NotNil(t, workflow)
			}
		})
	}
}

func TestRegistry_RegisterVersionedWorkflow_Conflict(t *testing.T) {
	r := New()

	// Register first version
	err := r.RegisterVersionedWorkflow(reg_workflow_v1, "1.0.0")
	require.NoError(t, err)

	// Try to register same version again - should fail
	err = r.RegisterVersionedWorkflow(reg_workflow_v1, "1.0.0")
	var wantErr *ErrVersionedWorkflowAlreadyRegistered
	require.ErrorAs(t, err, &wantErr)

	// Register different version - should succeed
	err = r.RegisterVersionedWorkflow(reg_workflow_v2, "2.0.0")
	require.NoError(t, err)

	// Register same workflow with custom name and same version - should succeed
	err = r.RegisterVersionedWorkflow(reg_workflow_v1, "1.0.0", WithName("CustomWorkflow"))
	require.NoError(t, err)

	// Try to register same custom name and version again - should fail
	err = r.RegisterVersionedWorkflow(reg_workflow_v1, "1.0.0", WithName("CustomWorkflow"))
	require.ErrorAs(t, err, &wantErr)
}

func TestRegistry_GetVersionedWorkflow(t *testing.T) {
	r := New()

	// Register multiple versions of the same workflow
	err := r.RegisterVersionedWorkflow(reg_workflow_v1, "1.0.0")
	require.NoError(t, err)

	err = r.RegisterVersionedWorkflow(reg_workflow_v2, "2.0.0")
	require.NoError(t, err)

	// Test successful retrieval
	workflow1, err := r.GetVersionedWorkflow("reg_workflow_v1", "1.0.0")
	require.NoError(t, err)
	require.NotNil(t, workflow1)

	workflow2, err := r.GetVersionedWorkflow("reg_workflow_v2", "2.0.0")
	require.NoError(t, err)
	require.NotNil(t, workflow2)

	// Test workflow not found
	_, err = r.GetVersionedWorkflow("nonexistent", "1.0.0")
	var notFoundErr *ErrWorkflowVersionNotFound
	require.ErrorAs(t, err, &notFoundErr)

	// Test version not found
	_, err = r.GetVersionedWorkflow("reg_workflow_v1", "3.0.0")
	require.ErrorAs(t, err, &notFoundErr)
}

func TestRegistry_VersionedWorkflow_BackwardCompatibility(t *testing.T) {
	r := New()

	// Register regular workflow
	err := r.RegisterWorkflow(reg_workflow1)
	require.NoError(t, err)

	// Register versioned workflow with same name
	err = r.RegisterVersionedWorkflow(reg_workflow_v1, "1.0.0")
	require.NoError(t, err)

	// Both should be retrievable independently
	regularWorkflow, err := r.GetWorkflow("reg_workflow1")
	require.NoError(t, err)
	require.NotNil(t, regularWorkflow)

	versionedWorkflow, err := r.GetVersionedWorkflow("reg_workflow_v1", "1.0.0")
	require.NoError(t, err)
	require.NotNil(t, versionedWorkflow)
}