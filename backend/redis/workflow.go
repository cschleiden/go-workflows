package redis

import (
	"context"
	"encoding/json"
	"log"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/pkg/errors"
)

func (rb *redisBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	// TODO: Check for timer events, and add them to pending events if required

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
	msgs, err := rb.rdb.XRange(ctx, historyKey(instanceTask.ID), "-", "+").Result()
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

	msgs, err = rb.rdb.XRange(ctx, pendingEventsKey(instanceTask.ID), "-", "+").Result()
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

	return &task.Workflow{
		ID:               instanceTask.TaskID,
		WorkflowInstance: core.NewWorkflowInstance(instanceTask.ID, instanceState.ExecutionID),
		History:          historyEvents,
		NewEvents:        newEvents,
	}, nil
}

func (rb *redisBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance core.WorkflowInstance) error {
	return rb.workflowQueue.Extend(ctx, taskID)
}

func (rb *redisBackend) CompleteWorkflowTask(ctx context.Context, taskID string, instance core.WorkflowInstance, state backend.WorkflowState, executedEvents []history.Event, activityEvents []history.Event, workflowEvents []history.WorkflowEvent) error {
	// Add executed events to the history
	for _, executedEvent := range executedEvents {
		if err := addEventToStream(ctx, rb.rdb, historyKey(instance.GetInstanceID()), &executedEvent); err != nil {
			return err
		}
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
			if err := createInstance(ctx, rb.rdb, targetInstance, true); err != nil {
				return err
			}
		}

		// Insert pending events for target instance
		for _, event := range events {
			if err := addEventToStream(ctx, rb.rdb, pendingEventsKey(targetInstance.GetInstanceID()), &event); err != nil {
				return err
			}
		}

		// TODO: Delay unlocking the current instance. Can we find a better way here?
		if targetInstance != instance {
			if _, err := rb.workflowQueue.Enqueue(ctx, targetInstance.GetInstanceID(), nil); err != nil {
				if err != taskqueue.ErrTaskAlreadyInQueue {
					return errors.Wrap(err, "could not add instance to locked instances set")
				}
			}
		}
	}

	// Update instance state with last message
	instanceState, err := readInstance(ctx, rb.rdb, instance.GetInstanceID())
	if err != nil {
		return errors.Wrap(err, "could not read workflow instance")
	}

	instanceState.State = state

	if err := updateInstance(ctx, rb.rdb, instance.GetInstanceID(), instanceState); err != nil {
		return errors.Wrap(err, "could not update workflow instance")
	}

	// Store activity data
	for _, activityEvent := range activityEvents {
		if _, err := rb.activityQueue.Enqueue(ctx, activityEvent.ID, &activityData{
			InstanceID: instance.GetInstanceID(),
			ID:         activityEvent.ID,
			Event:      activityEvent,
		}); err != nil {
			return errors.Wrap(err, "could not queue activity task")
		}
	}

	// Complete workflow task and unlock instance
	if err := rb.workflowQueue.Complete(ctx, taskID); err != nil {
		return errors.Wrap(err, "could not complete workflow task")
	}

	// If there are pending events, enqueue the instance again
	pendingCount, err := rb.rdb.XLen(ctx, pendingEventsKey(instance.GetInstanceID())).Result()
	if err != nil {
		return errors.Wrap(err, "could not read event stream")
	}

	if state != backend.WorkflowStateFinished && pendingCount > 0 {
		if _, err := rb.workflowQueue.Enqueue(ctx, instance.GetInstanceID(), nil); err != nil {
			if err != taskqueue.ErrTaskAlreadyInQueue {
				return errors.Wrap(err, "could not queue workflow")
			}
		}
	}

	log.Println("Unlocked workflow task", instance.GetInstanceID())

	return nil
}

func (rb *redisBackend) addWorkflowInstanceEvent(ctx context.Context, instance core.WorkflowInstance, event history.Event) error {
	// Add event to pending events for instance
	if err := addEventToStream(ctx, rb.rdb, pendingEventsKey(instance.GetInstanceID()), &event); err != nil {
		return err
	}

	// Queue workflow task
	if _, err := rb.workflowQueue.Enqueue(ctx, instance.GetInstanceID(), nil); err != nil {
		if err != taskqueue.ErrTaskAlreadyInQueue {
			return errors.Wrap(err, "could not queue workflow")
		}
	}

	return nil
}
