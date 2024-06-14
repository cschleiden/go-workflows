package backend

import (
	"time"
)

type RemovalOptions struct {
	FinishedBefore time.Time
}

type RemovalOption func(o *RemovalOptions)

func RemoveFinishedBefore(t time.Time) RemovalOption {
	return func(o *RemovalOptions) {
		o.FinishedBefore = t
	}
}
