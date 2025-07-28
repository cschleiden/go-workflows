package analyzer

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAll(t *testing.T) {
	newPlugin, err := register.GetPlugin("goworkflows")
	require.NoError(t, err)

	plugin, err := newPlugin(map[string]any{
		"checkprivatereturnvalues": true,
	})
	require.NoError(t, err)

	analyzers, err := plugin.BuildAnalyzers()
	require.NoError(t, err)

	analysistest.Run(t, analysistest.TestData(), analyzers[0], "p", "q")
}

func TestComplex(t *testing.T) {
	newPlugin, err := register.GetPlugin("goworkflows")
	require.NoError(t, err)

	plugin, err := newPlugin(map[string]any{
		"checkprivatereturnvalues": true,
	})
	require.NoError(t, err)

	analyzers, err := plugin.BuildAnalyzers()
	require.NoError(t, err)

	result := analysistest.Run(t, analysistest.TestData(), analyzers[0], "q")
	for _, r := range result {
		require.NoError(t, r.Err)
		require.Equal(t, 1, len(r.Diagnostics))
	}
}
