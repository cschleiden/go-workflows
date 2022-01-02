package converter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssignValue_Payload(t *testing.T) {
	payload, _ := DefaultConverter.To(42)
	var r int

	AssignValue(DefaultConverter, payload, &r)

	require.Equal(t, 42, r)
}

func TestAssignValue_Value(t *testing.T) {
	var r int

	AssignValue(DefaultConverter, 42, &r)

	require.Equal(t, 42, r)
}
