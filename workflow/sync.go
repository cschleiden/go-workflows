package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
)

type Future = sync.Future
type Channel = sync.Channel
type Context = sync.Context
type WaitGroup = sync.WaitGroup

var Canceled = sync.Canceled
