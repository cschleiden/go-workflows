package test

import (
	"context"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/history"
)

type TestBackend interface {
	backend.Backend

	GetFutureEvents(ctx context.Context) ([]history.Event, error)
}
