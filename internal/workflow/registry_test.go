package workflow

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

func Test_getFunctionName(t *testing.T) {
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
			got := getFunctionName(tt.i)
			require.Equal(t, tt.want, got)
		})
	}
}
