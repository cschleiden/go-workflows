package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/go-redis/redis/v8"
)

type futureEvent struct {
	Instance *core.WorkflowInstance `json:"instance,omitempty"`
	Event    *history.Event         `json:"event,omitempty"`
}

// Return all events with a visibleAt timestamp in the past. Also remove them from the set
// KEYS[1] - future event set key
// ARGV[1] - current timestamp for zrange
var futureEventsCmd = redis.NewScript(`
	local events = redis.call("ZRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
	if events ~= false and #events ~= 0 then
		redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
	end
	return events
`)

func (rb *redisBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	// Check for future events
	now := time.Now().Unix()
	nowStr := strconv.Itoa(int(now))

	result, err := futureEventsCmd.Run(ctx, rb.rdb, []string{futureEventsKey()}, nowStr).Result()
	if err != nil {
		return nil, fmt.Errorf("checking future events: %w", err)
	}

	for _, eventR := range result.([]interface{}) {
		eventStr := eventR.(string)
		var futureEvent futureEvent
		if err := json.Unmarshal([]byte(eventStr), &futureEvent); err != nil {
			return nil, fmt.Errorf("unmarshaling event: %w", err)
		}

		instanceState, err := readInstance(ctx, rb.rdb, futureEvent.Instance.InstanceID)
		if err != nil {
			if err == backend.ErrInstanceNotFound {
				rb.options.Logger.Debug("Ignoring future event for non-existing instance", "instance_id", futureEvent.Instance.InstanceID, "event_id", futureEvent.Event.ID)
				continue
			} else {
				return nil, fmt.Errorf("reading instance: %w", err)
			}
		}

		if instanceState.State != backend.WorkflowStateActive {
			rb.options.Logger.Debug("Ignoring future event for already completed instance", "instance_id", futureEvent.Instance.InstanceID, "event_id", futureEvent.Event.ID)
			continue
		}

		msgID, err := addEventToStream(ctx, rb.rdb, pendingEventsKey(futureEvent.Instance.InstanceID), futureEvent.Event)
		if err != nil {
			return nil, fmt.Errorf("adding future event to stream: %w", err)
		}

		// Instance now has at least one pending event, try to queue task
		if _, err := rb.workflowQueue.Enqueue(ctx, futureEvent.Instance.InstanceID, &workflowTaskData{
			LastPendingEventMessageID: *msgID,
		}); err != nil {
			if err != taskqueue.ErrTaskAlreadyInQueue {
				return nil, fmt.Errorf("queueing workflow task: %w", err)
			}
		}
	}

	// Try to get a workflow task
	instanceTask, err := rb.workflowQueue.Dequeue(ctx, rb.options.WorkflowLockTimeout, rb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if instanceTask == nil {
		return nil, nil
	}

	instanceState, err := readInstance(ctx, rb.rdb, instanceTask.ID)
	if err != nil {
		return nil, fmt.Errorf("reading workflow instance: %w", err)
	}

	// New Events
	newEvents := make([]history.Event, 0)

	msgs, err := rb.rdb.XRange(ctx, pendingEventsKey(instanceTask.ID), "-", instanceTask.Data.LastPendingEventMessageID).Result()
	if err != nil {
		return nil, fmt.Errorf("reading event stream: %w", err)
	}

	for _, msg := range msgs {
		var event history.Event

		if err := json.Unmarshal([]byte(msg.Values["event"].(string)), &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event: %w", err)
		}

		newEvents = append(newEvents, event)
	}

	return &task.Workflow{
		ID:               instanceTask.TaskID,
		WorkflowInstance: instanceState.Instance,
		LastSequenceID:   instanceState.LastSequenceID,
		NewEvents:        newEvents,
	}, nil
}

func (rb *redisBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error {
	return rb.workflowQueue.Extend(ctx, taskID)
}

// Remove all pending events before (and including) a given message id
// KEYS[1] - pending events stream key
// ARGV[1] - message id
var removePendingEventsCmd = redis.NewScript(`
	local trimmed = redis.call("XTRIM", KEYS[1], "MINID", ARGV[1])
	local deleted = redis.call("XDEL", KEYS[1], ARGV[1])
	local removed =  trimmed + deleted
	return removed
`)

func (rb *redisBackend) CompleteWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance, state backend.WorkflowState, executedEvents []history.Event, activityEvents []history.Event, workflowEvents []history.WorkflowEvent) error {
	task, err := rb.workflowQueue.Data(ctx, taskID)
	if err != nil {
		return fmt.Errorf("getting workflow task: %w", err)
	}

	// Add executed events to the history
	// TODO: Use pipeline
	for _, executedEvent := range executedEvents {
		if _, err := addEventToStream(ctx, rb.rdb, historyKey(instance.InstanceID), &executedEvent); err != nil {
			return err
		}
	}

	// Send new workflow events to the respective streams
	groupedEvents := make(map[*workflow.Instance][]history.Event)
	for _, m := range workflowEvents {
		if _, ok := groupedEvents[m.WorkflowInstance]; !ok {
			groupedEvents[m.WorkflowInstance] = []history.Event{}
		}

		groupedEvents[m.WorkflowInstance] = append(groupedEvents[m.WorkflowInstance], m.HistoryEvent)
	}

	for targetInstance, events := range groupedEvents {
		if instance.InstanceID != targetInstance.InstanceID {
			// Instance might not exist, try to create a new instance ignoring any duplicates
			if err := createInstance(ctx, rb.rdb, targetInstance, true); err != nil {
				return err
			}
		}

		// Insert pending events for target instance
		var lastPendingMessageID *string

		// TODO: use pipelines
		for _, event := range events {
			if event.VisibleAt != nil {
				// Add future events
				if err := addFutureEvent(ctx, rb.rdb, targetInstance, &event); err != nil {
					return err
				}
			} else {
				// Add pending event to stream
				lastPendingMessageID, err = addEventToStream(ctx, rb.rdb, pendingEventsKey(targetInstance.InstanceID), &event)
				if err != nil {
					return err
				}
			}
		}

		// If any pending message was added, try to queue workflow task
		if lastPendingMessageID != nil && targetInstance != instance {
			if _, err := rb.workflowQueue.Enqueue(ctx, targetInstance.InstanceID, &workflowTaskData{
				LastPendingEventMessageID: *lastPendingMessageID,
			}); err != nil {
				if err != taskqueue.ErrTaskAlreadyInQueue {
					return fmt.Errorf("adding instance to locked instances set: %w", err)
				}
			}
		}
	}

	// Update instance state with last message
	instanceState, err := readInstance(ctx, rb.rdb, instance.InstanceID)
	if err != nil {
		return fmt.Errorf("reading workflow instance: %w", err)
	}

	instanceState.State = state
	instanceState.LastSequenceID = executedEvents[len(executedEvents)-1].SequenceID

	if err := updateInstance(ctx, rb.rdb, instance.InstanceID, instanceState); err != nil {
		return fmt.Errorf("updating workflow instance: %w", err)
	}

	// Store activity data
	for _, activityEvent := range activityEvents {
		if _, err := rb.activityQueue.Enqueue(ctx, activityEvent.ID, &activityData{
			Instance: instance,
			ID:       activityEvent.ID,
			Event:    activityEvent,
		}); err != nil {
			return fmt.Errorf("queueing activity task: %w", err)
		}
	}

	// Remove executed pending events
	_, err = removePendingEventsCmd.Run(ctx, rb.rdb, []string{pendingEventsKey(instance.InstanceID)}, task.Data.LastPendingEventMessageID).Result()
	if err != nil {
		return fmt.Errorf("removing pending events: %w", err)
	}
	// log.Printf("Removed %v pending events", removed)

	// Complete workflow task and unlock instance
	if err := rb.workflowQueue.Complete(ctx, taskID); err != nil {
		return fmt.Errorf("completing workflow task: %w", err)
	}

	// If there are pending events, queue the instance again
	msgIDs, err := rb.rdb.XRevRangeN(ctx, pendingEventsKey(instance.InstanceID), "+", "-", 1).Result()
	if err != nil {
		return fmt.Errorf("reading event stream: %w", err)
	}

	if state != backend.WorkflowStateFinished && len(msgIDs) > 0 {
		if _, err := rb.workflowQueue.Enqueue(ctx, instance.InstanceID, &workflowTaskData{
			LastPendingEventMessageID: msgIDs[0].ID,
		}); err != nil {
			if err != taskqueue.ErrTaskAlreadyInQueue {
				return fmt.Errorf("queueing workflow: %w", err)
			}
		}
	}

	return nil
}

func (rb *redisBackend) addWorkflowInstanceEvent(ctx context.Context, instance *core.WorkflowInstance, event *history.Event) error {
	// Add event to pending events for instance
	msgID, err := addEventToStream(ctx, rb.rdb, pendingEventsKey(instance.InstanceID), event)
	if err != nil {
		return err
	}

	// Queue workflow task
	if _, err := rb.workflowQueue.Enqueue(ctx, instance.InstanceID, &workflowTaskData{
		LastPendingEventMessageID: *msgID,
	}); err != nil {
		if err != taskqueue.ErrTaskAlreadyInQueue {
			return fmt.Errorf("queueing workflow: %w", err)
		}
	}

	return nil
}
