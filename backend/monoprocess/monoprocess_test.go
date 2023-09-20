package monoprocess

import (
	"context"
	"errors"
	"testing"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/sqlite"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/internal/history"
)

func Test_MonoprocessBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.BackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		options = append(options, backend.WithStickyTimeout(0))

		return NewMonoprocessBackend(sqlite.NewInMemoryBackend(options...))
	}, nil)
}

func Test_EndToEndMonoprocessBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		// Disable sticky workflow behavior for the test execution
		options = append(options, backend.WithStickyTimeout(0))

		return NewMonoprocessBackend(sqlite.NewInMemoryBackend(options...))
	}, nil)
}

var _ test.TestBackend = (*monoprocessBackend)(nil)

func (b *monoprocessBackend) GetFutureEvents(ctx context.Context) ([]*history.Event, error) {
	if testBackend, ok := b.Backend.(test.TestBackend); ok {
		return testBackend.GetFutureEvents(ctx)
	}
	return nil, errors.New("not implemented")
}
