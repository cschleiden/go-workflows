package redis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_newKeys(t *testing.T) {
	t.Run("WithEmptyPrefix", func(t *testing.T) {
		k := newKeys("")
		require.Equal(t, "", k.prefix)
	})

	t.Run("WithNonEmptyPrefixWithoutColon", func(t *testing.T) {
		k := newKeys("prefix")
		require.Equal(t, "prefix:", k.prefix)
	})

	t.Run("WithNonEmptyPrefixWithColon", func(t *testing.T) {
		k := newKeys("prefix:")
		require.Equal(t, "prefix:", k.prefix)
	})
}
