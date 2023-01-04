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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Name(tt.i)
			require.Equal(t, tt.want, got)
		})
	}
}
