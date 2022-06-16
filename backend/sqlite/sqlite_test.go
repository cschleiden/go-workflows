package sqlite

import (
	"context"
	"testing"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/internal/history"
)

func Test_SqliteBackend(t *testing.T) {
	test.BackendTest(t, func() test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		return NewInMemoryBackend(backend.WithStickyTimeout(0))
	}, nil)
}

func Test_EndToEndSqliteBackend(t *testing.T) {
	test.EndToEndBackendTest(t, func() test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		return NewInMemoryBackend(backend.WithStickyTimeout(0))
	}, nil)
}

var _ test.TestBackend = (*sqliteBackend)(nil)

func (sb *sqliteBackend) GetFutureEvents(ctx context.Context) ([]history.Event, error) {
	// TODO: TESTING: Implement
	return nil, nil
}
