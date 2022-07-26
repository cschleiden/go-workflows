package args

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ticctech/go-workflows/internal/converter"
	"github.com/ticctech/go-workflows/internal/payload"
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
