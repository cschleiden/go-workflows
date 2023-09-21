package args

import (
	"context"
	"reflect"
	"testing"

	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/converter"
	"github.com/stretchr/testify/require"
)

func TestInputsToArgs(t *testing.T) {
	type args struct {
		fn     interface{}
		inputs []interface{}
	}
	tests := []struct {
		name       string
		args       args
		addContext bool
		wantErr    bool
		err        string
	}{
		{
			name: "just context",
			args: args{
				fn:     func(context.Context) error { return nil },
				inputs: []interface{}{},
			},
			addContext: true,
		},
		{
			name: "arguments with context",
			args: args{
				fn:     func(context.Context, int, string) error { return nil },
				inputs: []interface{}{42, ""},
			},
			addContext: true,
		},
		{
			name: "mismatched argument count - too many",
			args: args{
				fn:     func(int, string) error { return nil },
				inputs: []interface{}{42, "", 13},
			},
			wantErr: true,
			err:     "mismatched argument count: expected 2, got 3",
		},
		{
			name: "mismatched argument count - too few",
			args: args{
				fn:     func(int, string) error { return nil },
				inputs: []interface{}{42},
			},
			wantErr: true,
			err:     "mismatched argument count: expected 2, got 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := make([]payload.Payload, 0)
			for _, input := range tt.args.inputs {
				p, err := converter.DefaultConverter.To(input)
				require.NoError(t, err)

				inputs = append(inputs, p)
			}

			args, addContext, err := InputsToArgs(converter.DefaultConverter, reflect.ValueOf(tt.args.fn), inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("InputsToArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				require.EqualError(t, err, tt.err)
				require.Equal(t, tt.addContext, addContext)
			} else {
				if addContext {
					// Skip the first argument, it will be filled with the context later
					args = args[1:]
				}

				argValues := make([]interface{}, 0)
				for _, arg := range args {
					argValues = append(argValues, arg.Interface())
				}
			}
		})
	}
}

func intReturn() (int, error) {
	return 0, nil
}

func stringReturn() (string, error) {
	return "", nil
}

func errorReturn() error {
	return nil
}

func TestReturnTypeMatch(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
		want string
	}{
		{
			name: "int match",
			fn: func() error {
				return ReturnTypeMatch[int](intReturn)
			},
			want: "",
		},
		{
			name: "string match",
			fn: func() error {
				return ReturnTypeMatch[string](stringReturn)
			},
			want: "",
		},
		{
			name: "int mismatch",
			fn: func() error {
				return ReturnTypeMatch[string](intReturn)
			},
			want: "function must return string, got int",
		},
		{
			name: "no param",
			fn: func() error {
				return ReturnTypeMatch[any](errorReturn)
			},
			want: "",
		},
		{
			name: "no param mismatch",
			fn: func() error {
				return ReturnTypeMatch[int](errorReturn)
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if tt.want == "" {
				require.NoError(t, got)
			} else {
				require.Error(t, got)
				require.Equal(t, tt.want, got.Error())
			}
		})
	}
}

func intParam(int) {
}

func stringParam(string) {
}

func interfaceParam(string, interface{}, int) {
}

func mixedParams(context.Context, int, string) {
}

func TestParamsMatch(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
		want string
	}{
		{
			name: "int match",
			fn: func() error {
				return ParamsMatch(intParam, 42)
			},
			want: "",
		},
		{
			name: "int mismatch",
			fn: func() error {
				return ParamsMatch(intParam, "")
			},
			want: "mismatched argument type: expected int, got string",
		},
		{
			name: "string mismatch",
			fn: func() error {
				return ParamsMatch(stringParam, 42)
			},
			want: "mismatched argument type: expected string, got int",
		},
		{
			name: "interface{} ignored",
			fn: func() error {
				return ParamsMatch(interfaceParam, "", 23, 42)
			},
			want: "",
		},
		{
			name: "mixed params",
			fn: func() error {
				return ParamsMatch(mixedParams, 42, "")
			},
			want: "",
		},
		{
			name: "context",
			fn: func() error {
				return ParamsMatch(mixedParams, 42, "", 23)
			},
			want: "mismatched argument count: expected 2, got 3",
		},
		{
			name: "mixed params - wrong params",
			fn: func() error {
				return ParamsMatch(mixedParams, "", 42)
			},
			want: "mismatched argument type: expected int, got string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if tt.want == "" {
				require.NoError(t, got)
			} else {
				require.Error(t, got)
				require.Equal(t, tt.want, got.Error())
			}
		})
	}
}
