package test

import (
	"context"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/diag"
)

type TestBackend interface {
	backend.Backend
	diag.Backend

	GetFutureEvents(ctx context.Context) ([]*history.Event, error)
}
