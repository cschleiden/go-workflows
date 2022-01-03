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

func TestAssignValue_Zero(t *testing.T) {
	r := int(42)
	AssignValue(DefaultConverter, nil, &r)
	require.Equal(t, 0, r)

	b := true
	AssignValue(DefaultConverter, nil, &b)
	require.False(t, b)
}
