package sqlite

import (
	"testing"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/test"
)

func Test_SqliteBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.BackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		options = append(options, backend.WithStickyTimeout(0))

		return NewInMemoryBackend(options...)
	}, nil)
}

func Test_EndToEndSqliteBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		options = append(options, backend.WithStickyTimeout(0))

		return NewInMemoryBackend(options...)
	}, nil)
}
