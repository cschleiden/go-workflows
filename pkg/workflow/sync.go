package workflow

import (
	"github.com/cschleiden/go-workflows/internal/sync"
)

type Future = sync.Future
type Selector = sync.Selector
type Channel = sync.Channel
type Context = sync.Context

var Canceled = sync.Canceled
