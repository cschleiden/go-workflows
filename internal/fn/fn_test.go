package fn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type foo struct{}

func (f *foo) DoSomething(ctx context.Context) error {
	return nil
}

func (f *foo) bar(ctx context.Context) error {
	return nil
}

var f foo

func bar(_ int) {
}

func Test_FuncName(t *testing.T) {
	tests := []struct {
		name string
		i    interface{}
		want string
	}{
		{
			name: "function",
			i:    bar,
			want: "github.com/cschleiden/go-workflows/internal/fn.bar",
		},
		{
			name: "struct method",
			i:    f.bar,
			want: "github.com/cschleiden/go-workflows/internal/fn.(*foo).bar",
		},
		{
			name: "exported struct method",
			i:    f.DoSomething,
			want: "github.com/cschleiden/go-workflows/internal/fn.(*foo).DoSomething",
		},
		{
			name: "anonymous function",
			i:    func() {},
			want: "github.com/cschleiden/go-workflows/internal/fn.Test_FuncName.func1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuncName(tt.i)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_StructName(t *testing.T) {
	tests := []struct {
		name string
		i    interface{}
		want string
	}{
		{
			name: "value",
			i:    foo{},
			want: "github.com/cschleiden/go-workflows/internal/fn.foo",
		},
		{
			name: "pointer",
			i:    &foo{},
			want: "github.com/cschleiden/go-workflows/internal/fn.(*foo)",
		},
		{
			name: "inline value",
			i:    struct{ foo int }{},
			want: "struct { foo int }",
		},
		{
			name: "inline pointer",
			i:    &struct{ foo int }{},
			want: "*struct { foo int }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StructName(tt.i)
			require.Equal(t, tt.want, got)
		})
	}
}
