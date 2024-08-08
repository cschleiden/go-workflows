package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/workflows"
)

const expirationWorkflowInstanceID = "expiration"

// StartAutoExpiration starts a system workflow that will automatically expire workflow instances.
//
// The workflow will run every `delay` and remove all workflow instances finished before Now() - `delay`.
func (c *Client) StartAutoExpiration(ctx context.Context, delay time.Duration) error {
	if !c.backend.FeatureSupported(backend.Feature_Expiration) {
		return &backend.ErrNotSupported{}
	}

	_, err := c.CreateWorkflowInstance(ctx, WorkflowInstanceOptions{
		InstanceID: expirationWorkflowInstanceID,
	}, workflows.ExpireWorkflowInstances, delay)
	if err != nil {
		if errors.Is(err, backend.ErrInstanceAlreadyExists) {
			err = c.SignalWorkflow(ctx, expirationWorkflowInstanceID, workflows.UpdateExpirationSignal, delay)
			if err != nil {
				return fmt.Errorf("updating expiration workflow: %w", err)
			}

			return nil
		}

		return fmt.Errorf("starting expiration workflow: %w", err)
	}

	return nil
}
