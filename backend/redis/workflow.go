package redis

import (
	"context"
	"encoding/json"
	"log"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

func (rb *redisBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	instanceTask, err := rb.workflowQueue.Dequeue(ctx, rb.options.WorkflowLockTimeout, rb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if instanceTask == nil {
		return nil, nil
	}

	instanceState, err := readInstance(ctx, rb.rdb, instanceTask.ID)
	if err != nil {
		return nil, errors.Wrap(err, "could not read workflow instance")
	}

	// History
	cmd := rb.rdb.XRange(ctx, historyKey(instanceTask.ID), "-", "+")
	msgs, err := cmd.Result()
	if err != nil {
		return nil, errors.Wrap(err, "could not read event stream")
	}

	historyEvents := make([]history.Event, 0)

	for _, msg := range msgs {
		var event history.Event

		if err := json.Unmarshal([]byte(msg.Values["event"].(string)), &event); err != nil {
			return nil, errors.Wrap(err, "could not unmarshal event")
		}

		historyEvents = append(historyEvents, event)
	}

	// New Events
	newEvents := make([]history.Event, 0)

	cmd = rb.rdb.XRange(ctx, pendingEventsKey(instanceTask.ID), "-", "+")
	msgs, err = cmd.Result()
	if err != nil {
		return nil, errors.Wrap(err, "could not read event stream")
	}

	for _, msg := range msgs {
		var event history.Event

		if err := json.Unmarshal([]byte(msg.Values["event"].(string)), &event); err != nil {
			return nil, errors.Wrap(err, "could not unmarshal event")
		}

		newEvents = append(newEvents, event)
	}

	// Remove all pending events
	rb.rdb.XTrim(ctx, pendingEventsKey(instanceTask.ID), 0)

	log.Println("Returned task for ", instanceTask)

	return &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance(instanceTask.ID, instanceState.ExecutionID),
		History:          historyEvents,
		NewEvents:        newEvents,
	}, nil
}

func (rb *redisBackend) ExtendWorkflowTask(ctx context.Context, instance core.WorkflowInstance) error {
	// TODO: Extend lock for instance

	panic("unimplemented")
}

func (rb *redisBackend) CompleteWorkflowTask(ctx context.Context, instance core.WorkflowInstance, state backend.WorkflowState, executedEvents []history.Event, activityEvents []history.Event, workflowEvents []history.WorkflowEvent) error {

	// Add events to stream
	var lastMessageID string

	for _, executedEvent := range executedEvents {
		// TODO: Use pipeline
		eventData, err := json.Marshal(executedEvent)
		if err != nil {
			return err
		}

		cmd := rb.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: historyKey(instance.GetInstanceID()),
			ID:     "*",
			Values: map[string]interface{}{
				"event": string(eventData),
			},
		})
		id, err := cmd.Result()
		if err != nil {
			return errors.Wrap(err, "could not create event stream")
		}

		lastMessageID = id
	}

	// Send new events to the respective streams
	groupedEvents := make(map[workflow.Instance][]history.Event)
	for _, m := range workflowEvents {
		if _, ok := groupedEvents[m.WorkflowInstance]; !ok {
			groupedEvents[m.WorkflowInstance] = []history.Event{}
		}

		groupedEvents[m.WorkflowInstance] = append(groupedEvents[m.WorkflowInstance], m.HistoryEvent)
	}

	for targetInstance, events := range groupedEvents {
		if instance.GetInstanceID() != targetInstance.GetInstanceID() {
			// Create new instance
			// TODO: Support creating sub-workflows
			panic("not implemented")
		}

		// Insert pending events for target instance
		for _, event := range events {
			// TODO: Use pipeline
			eventData, err := json.Marshal(event)
			if err != nil {
				return err
			}

			cmd := rb.rdb.XAdd(ctx, &redis.XAddArgs{
				Stream: pendingEventsKey(targetInstance.GetInstanceID()),
				ID:     "*",
				Values: map[string]interface{}{
					"event": string(eventData),
				},
			})
			_, err = cmd.Result()
			if err != nil {
				return errors.Wrap(err, "could not create event stream")
			}
		}

		// TODO: Delay unlocking the current instance. Can we find a better way here?
		if targetInstance != instance {
			if err := rb.workflowQueue.Enqueue(ctx, targetInstance.GetInstanceID()); err != nil {
				return errors.Wrap(err, "could not add instance to locked instances set")
			}
		}
	}

	// Update instance state with last message
	instanceState, err := readInstance(ctx, rb.rdb, instance.GetInstanceID())
	if err != nil {
		return errors.Wrap(err, "could not read workflow instance")
	}

	instanceState.State = state
	instanceState.LastMessageID = lastMessageID

	if err := updateInstance(ctx, rb.rdb, instance.GetInstanceID(), instanceState); err != nil {
		return errors.Wrap(err, "could not update workflow instance")
	}

	// Store activity data
	// TODO: Use pipeline?
	for _, activityEvent := range activityEvents {
		if err := storeActivity(ctx, rb.rdb, &ActivityData{
			InstanceID: instance.GetInstanceID(),
			ID:         activityEvent.ID,
			Event:      activityEvent,
		}); err != nil {
			return errors.Wrap(err, "could not store activity data")
		}

		if err := rb.activityQueue.Enqueue(ctx, activityEvent.ID); err != nil {
			return errors.Wrap(err, "could not queue activity")
		}
	}

	// Unlock instance
	if err := rb.workflowQueue.Complete(ctx, instance.GetInstanceID()); err != nil {
		return errors.Wrap(err, "could not complete workflow task")
	}

	log.Println("Unlocked", instance.GetInstanceID())

	return nil
}
