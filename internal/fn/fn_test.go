package fn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type foo struct {
}

func (f *foo) DoSomething(ctx context.Context) error {
	return nil
}

func (f *foo) bar(ctx context.Context) error {
	return nil
}

var f foo

func bar(_ int) {
}

func Test_GetFunctionName(t *testing.T) {
	tests := []struct {
		name string
		i    interface{}
		want string
	}{
		{
			name: "function",
			i:    bar,
			want: "bar",
		},
		{
			name: "struct method",
			i:    f.bar,
			want: "bar",
		},
		{
			name: "exported struct method",
			i:    f.DoSomething,
			want: "DoSomething",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Name(tt.i)
			require.Equal(t, tt.want, got)
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
				return ParamsMatch(intParam, 0, 42)
			},
			want: "",
		},
		{
			name: "int mismatch",
			fn: func() error {
				return ParamsMatch(intParam, 0, "")
			},
			want: "mismatched argument type: expected int, got string",
		},
		{
			name: "string mismatch",
			fn: func() error {
				return ParamsMatch(stringParam, 0, 42)
			},
			want: "mismatched argument type: expected string, got int",
		},
		{
			name: "interface{} ignored",
			fn: func() error {
				return ParamsMatch(interfaceParam, 0, "", 23, 42)
			},
			want: "",
		},
		{
			name: "mixed params",
			fn: func() error {
				return ParamsMatch(mixedParams, 1, 42, "")
			},
			want: "",
		},
		{
			name: "mixed params - no skip",
			fn: func() error {
				return ParamsMatch(mixedParams, 0, 42, "")
			},
			want: "mismatched argument count: expected 3, got 2",
		},
		{
			name: "mixed params - wrong params",
			fn: func() error {
				return ParamsMatch(mixedParams, 1, "", 42)
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
