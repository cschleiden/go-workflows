package workflow

import "github.com/cschleiden/go-workflows/internal/sync"

type Future[T any] = sync.Future[T]
