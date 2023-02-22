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
		fn   func() bool
		want bool
	}{
		{
			name: "int match",
			fn: func() bool {
				return ReturnTypeMatch[int](intReturn)
			},
			want: true,
		},
		{
			name: "string match",
			fn: func() bool {
				return ReturnTypeMatch[string](stringReturn)
			},
			want: true,
		},
		{
			name: "int mismatch",
			fn: func() bool {
				return ReturnTypeMatch[string](intReturn)
			},
		},
		{
			name: "no param",
			fn: func() bool {
				return ReturnTypeMatch[any](errorReturn)
			},
			want: true,
		},
		{
			name: "no param mismatch",
			fn: func() bool {
				return ReturnTypeMatch[int](errorReturn)
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			require.Equal(t, tt.want, got)
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
		fn   func() bool
		want bool
	}{
		{
			name: "int match",
			fn: func() bool {
				return ParamsMatch(intParam, 0, 42)
			},
			want: true,
		},
		{
			name: "int mismatch",
			fn: func() bool {
				return ParamsMatch(intParam, 0, "")
			},
			want: false,
		},
		{
			name: "string mismatch",
			fn: func() bool {
				return ParamsMatch(stringParam, 0, 42)
			},
			want: false,
		},
		{
			name: "interface{} ignored",
			fn: func() bool {
				return ParamsMatch(interfaceParam, 0, "", 23, 42)
			},
			want: true,
		},
		{
			name: "mixed params",
			fn: func() bool {
				return ParamsMatch(mixedParams, 1, 42, "")
			},
			want: true,
		},
		{
			name: "mixed params - no skip",
			fn: func() bool {
				return ParamsMatch(mixedParams, 0, 42, "")
			},
			want: false,
		},
		{
			name: "mixed params - wrong params",
			fn: func() bool {
				return ParamsMatch(mixedParams, 1, "", 42)
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			require.Equal(t, tt.want, got)
		})
	}
}
