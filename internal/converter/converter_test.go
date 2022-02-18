package converter

import (
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/internal/payload"
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

	var v payload.Payload
	b = true
	AssignValue(DefaultConverter, v, &b)
	require.False(t, b)
}

func TestAssignValue_Time(t *testing.T) {
	i := time.Now()
	payload, _ := DefaultConverter.To(i)
	var r time.Time

	AssignValue(DefaultConverter, payload, &r)

	require.True(t, i.Equal(r))
}
