package client

import (
	"context"

	"github.com/cschleiden/go-workflows/backend"
)

func (c *client) GetStats(ctx context.Context) (*backend.Stats, error) {
	return c.backend.GetStats(ctx)
}
