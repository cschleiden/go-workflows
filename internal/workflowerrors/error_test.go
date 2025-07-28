package workflowerrors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_NewError_Nil(t *testing.T) {
	err := FromError(nil)
	require.Nil(t, err)
}

func Test_NewError_DoesNotWrapAgain(t *testing.T) {
	err := FromError(errors.New("foo"))

	err2 := FromError(err)
	require.NoError(t, errors.Unwrap(err2))
}

func Test_NewError_DoesWrap(t *testing.T) {
	input := errors.New("foo")
	e := FromError(input)

	var expectedType *Error
	require.ErrorAs(t, e, &expectedType)
	require.Error(t, e, input.Error())

	require.False(t, e.Permanent)
	require.NoError(t, e.Cause)
}

func Test_NewPermanentError(t *testing.T) {
	input := errors.New("foo")
	e := NewPermanentError(input)

	var expected *Error
	require.ErrorAs(t, e, &expected)
	require.Error(t, e, input.Error())

	require.True(t, e.Permanent)
	require.NoError(t, e.Cause)
}

func Test_RoundTrip(t *testing.T) {
	input := NewPanicError("foo")
	e := FromError(input)

	output := ToError(e)
	require.Equal(t, input, output)
}

func TestCanRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Error",
			err:  FromError(errors.New("foo")),
			want: true,
		},
		{
			name: "Permanent",
			err:  NewPermanentError(errors.New("foo")),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanRetry(tt.err); got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
