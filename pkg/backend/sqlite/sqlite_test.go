package sqlite

import (
	"testing"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/test"
)

func Test_SqliteBackend(t *testing.T) {
	test.TestBackend(t, test.Tester{
		New: func() backend.Backend {
			// Disable sticky workflow behavior for the test execution
			return NewInMemoryBackend(backend.WithStickyTimeout(0))
		},
	})
}
