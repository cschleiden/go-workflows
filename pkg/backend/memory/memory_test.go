package memory

import (
	"testing"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/test"
)

func Test_MemoryBackend(t *testing.T) {
	test.TestBackend(t, test.Tester{
		New: func() backend.Backend {
			return NewMemoryBackend()
		},
	})
}
