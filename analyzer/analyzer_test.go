package analyzer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAll(t *testing.T) {
	a := New()
	a.Flags.Set("checkprivatereturnvalues", "true")
	analysistest.Run(t, analysistest.TestData(), a, "p", "q")
}

func TestComplex(t *testing.T) {
	a := New()
	a.Flags.Set("checkprivatereturnvalues", "true")
	result := analysistest.Run(t, analysistest.TestData(), a, "q")
	for _, r := range result {
		require.NoError(t, r.Err)
		require.Equal(t, 1, len(r.Diagnostics))
	}
}
