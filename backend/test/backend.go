package test

import (
	"context"

	"github.com/ticctech/go-workflows/backend"
	"github.com/ticctech/go-workflows/internal/history"
)

type TestBackend interface {
	backend.Backend

	GetFutureEvents(ctx context.Context) ([]history.Event, error)
}
