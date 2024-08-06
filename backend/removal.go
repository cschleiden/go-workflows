package backend

import (
	"time"
)

type RemovalOptions struct {
	FinishedBefore time.Time
	BatchSize      int
}

var DefaultRemovalOptions = RemovalOptions{
	BatchSize: 100,
}

type RemovalOption func(o *RemovalOptions)

func RemoveFinishedBefore(t time.Time) RemovalOption {
	return func(o *RemovalOptions) {
		o.FinishedBefore = t
	}
}

func RemoveFinishedBatchSize(size int) RemovalOption {
	return func(o *RemovalOptions) {
		o.BatchSize = size
	}
}
