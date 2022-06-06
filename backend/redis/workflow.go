package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/redis/taskqueue"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
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
	local events = redis.call("ZRANGE", KEYS[1], "-inf", ARGV[1], "BYSCORE")
	if events ~= false and #events ~= 0 then
		redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
		return redis.call("MGET", unpack(events))
	end
`)

func (rb *redisBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	// TODO: Re-enable. Ensure no events get lost. Can we make adding to the pending event stream idempotent? Do we need an additional set for that?
	// pending-event `STREAM` for ordering, and `SET` for deduplication?

	// // Check for future events
	// now := time.Now().Unix()
	// nowStr := strconv.Itoa(int(now))

	// result, err := futureEventsCmd.Run(ctx, rb.rdb, []string{futureEventsKey()}, nowStr).Result()
	// if err != nil && err != redis.Nil {
	// 	return nil, fmt.Errorf("checking future events: %w", err)
	// }

	// if result != nil {
	// 	for _, eventR := range result.([]interface{}) {
	// 		eventStr := eventR.(string)
	// 		var futureEvent futureEvent
	// 		if err := json.Unmarshal([]byte(eventStr), &futureEvent); err != nil {
	// 			return nil, fmt.Errorf("unmarshaling event: %w", err)
	// 		}

	// 		instanceState, err := readInstance(ctx, rb.rdb, futureEvent.Instance.InstanceID)
	// 		if err != nil {
	// 			if err == backend.ErrInstanceNotFound {
	// 				rb.options.Logger.Debug("Ignoring future event for non-existing instance", "instance_id", futureEvent.Instance.InstanceID, "event_id", futureEvent.Event.ID)
	// 				continue
	// 			} else {
	// 				return nil, fmt.Errorf("reading instance: %w", err)
	// 			}
	// 		}

	// 		if instanceState.State != backend.WorkflowStateActive {
	// 			rb.options.Logger.Debug("Ignoring future event for already completed instance", "instance_id", futureEvent.Instance.InstanceID, "event_id", futureEvent.Event.ID)
	// 			continue
	// 		}

	// 		msgID, err := addEventToStream(ctx, rb.rdb, pendingEventsKey(futureEvent.Instance.InstanceID), futureEvent.Event)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("adding future event to stream: %w", err)
	// 		}

	// 		// Instance now has at least one pending event, try to queue task
	// 		if _, err := rb.workflowQueue.Enqueue(ctx, futureEvent.Instance.InstanceID, &workflowTaskData{
	// 			LastPendingEventMessageID: *msgID,
	// 		}); err != nil {
	// 			if err != taskqueue.ErrTaskAlreadyInQueue {
	// 				return nil, fmt.Errorf("queueing workflow task: %w", err)
	// 			}
	// 		}
	// 	}
	// }

	// Try to get a workflow task
	instanceTask, err := rb.workflowQueue.Dequeue(ctx, rb.rdb, rb.options.WorkflowLockTimeout, rb.options.BlockTimeout)
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

	msgs, err := rb.rdb.XRange(ctx, pendingEventsKey(instanceTask.ID), "-", "+").Result()
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
		CustomData:       msgs[len(msgs)-1].ID, // Id of last pending message in stream
	}, nil
}

func (rb *redisBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error {
	_, err := rb.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		return rb.workflowQueue.Extend(ctx, p, taskID)
	})

	return err
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

func (rb *redisBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *task.Workflow,
	instance *core.WorkflowInstance,
	state backend.WorkflowState,
	executedEvents []history.Event,
	activityEvents []history.Event,
	workflowEvents []history.WorkflowEvent,
) error {
	instanceState, err := readInstance(ctx, rb.rdb, instance.InstanceID)
	if err != nil {
		return err
	}

	// Check-point the workflow. We guarantee that no other worker is working on this workflow instance at this point via the
	// task queue, so we don't need to WATCH the keys, we just need to make sure all commands are executed atomically to prevent
	// a worker crashing in the middle of this execution.
	p := rb.rdb.TxPipeline()

	// Add executed events to the history
	if err := addEventsToStreamP(ctx, p, historyKey(instance.InstanceID), executedEvents); err != nil {
		return fmt.Errorf("serializing : %w", err)
	}

	// Send new workflow events to the respective streams
	groupedEvents := eventsByWorkflowInstance(workflowEvents)
	for targetInstance, events := range groupedEvents {
		if instance.InstanceID != targetInstance.InstanceID {
			// Instance might not exist, try to create a new instance ignoring any duplicates
			if err := createInstanceP(ctx, p, targetInstance, true); err != nil {
				return err
			}
		}

		// Insert pending events for target instance
		msgAdded := false

		for _, event := range events {
			switch event.Type {
			case history.EventType_TimerCanceled:
				removeFutureEventP(ctx, p, targetInstance, event)
			}

			if event.VisibleAt != nil {
				if err := addFutureEventP(ctx, p, targetInstance, event); err != nil {
					return err
				}
			} else {
				// Add pending event to stream
				if err := addEventToStreamP(ctx, p, pendingEventsKey(targetInstance.InstanceID), event); err != nil {
					return err
				}

				msgAdded = true
			}
		}

		// If any pending message was added, try to queue workflow task
		if targetInstance != instance && msgAdded {
			if err := rb.workflowQueue.Enqueue(ctx, p, targetInstance.InstanceID, &workflowTaskData{
				LastPendingEventMessageID: "", // TODO: Can we figure this out when picking up the task instead of adding it here?
			}); err != nil {
				if err != taskqueue.ErrTaskAlreadyInQueue {
					return fmt.Errorf("adding instance to locked instances set: %w", err)
				}
			}
		}
	}

	// Update instance state with last message
	// TODO: REDIS: Do this in one go? Use lua json functionality?
	// instanceState, err := readInstanceP(ctx, p, instance.InstanceID)
	// if err != nil {
	// 	return fmt.Errorf("reading workflow instance: %w", err)
	// }

	instanceState.State = state
	instanceState.LastSequenceID = executedEvents[len(executedEvents)-1].SequenceID

	if err := updateInstanceP(ctx, p, instance.InstanceID, instanceState); err != nil {
		return fmt.Errorf("updating workflow instance: %w", err)
	}

	// Store activity data
	for _, activityEvent := range activityEvents {
		if err := rb.activityQueue.Enqueue(ctx, p, activityEvent.ID, &activityData{
			Instance: instance,
			ID:       activityEvent.ID,
			Event:    activityEvent,
		}); err != nil {
			return fmt.Errorf("queueing activity task: %w", err)
		}
	}

	// Remove executed pending events
	lastPendingEventMessageID := task.CustomData.(string)
	removePendingEventsCmd.Run(ctx, p, []string{pendingEventsKey(instance.InstanceID)}, lastPendingEventMessageID)

	// Complete workflow task and unlock instance
	if err := rb.workflowQueue.Complete(ctx, p, task.ID); err != nil {
		return fmt.Errorf("completing workflow task: %w", err)
	}

	// If there are pending events, queue the instance again
	msgIDs, err := p.XRevRangeN(ctx, pendingEventsKey(instance.InstanceID), "+", "-", 1).Result()
	if err != nil {
		return fmt.Errorf("reading event stream: %w", err)
	}

	if state != backend.WorkflowStateFinished && len(msgIDs) > 0 {
		if err := rb.workflowQueue.Enqueue(ctx, p, instance.InstanceID, &workflowTaskData{
			LastPendingEventMessageID: msgIDs[0].ID,
		}); err != nil {
			if err != taskqueue.ErrTaskAlreadyInQueue {
				return fmt.Errorf("queueing workflow: %w", err)
			}
		}
	}

	// Commit transaction
	executedCmds, err := p.Exec(ctx)
	if err != nil {
		for _, cmd := range executedCmds {
			if cmdErr := cmd.Err(); cmdErr != nil {
				rb.Logger().Debug("redis command error", "cmd", cmd.FullName(), "cmdErr", cmdErr.Error())
			}
		}

		return fmt.Errorf("completing workflow task: %w", err)
	}

	return nil
}

func (rb *redisBackend) addWorkflowInstanceEventP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, event *history.Event) error {
	// Add event to pending events for instance
	if err := addEventToStreamP(ctx, p, pendingEventsKey(instance.InstanceID), event); err != nil {
		return err
	}

	// Queue workflow task
	if err := rb.workflowQueue.Enqueue(ctx, p, instance.InstanceID, &workflowTaskData{
		LastPendingEventMessageID: "", // TODO: REDIS: Can we get this at pick up time?
	}); err != nil {
		if err != taskqueue.ErrTaskAlreadyInQueue {
			return fmt.Errorf("queueing workflow: %w", err)
		}
	}

	return nil
}

func eventsByWorkflowInstance(events []history.WorkflowEvent) map[*core.WorkflowInstance][]*history.Event {
	groupedEvents := make(map[*core.WorkflowInstance][]*history.Event)

	for _, m := range events {
		if _, ok := groupedEvents[m.WorkflowInstance]; !ok {
			groupedEvents[m.WorkflowInstance] = []*history.Event{}
		}

		groupedEvents[m.WorkflowInstance] = append(groupedEvents[m.WorkflowInstance], &m.HistoryEvent)
	}

	return groupedEvents
}
