package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Find all due future events. For each event:
// - Look up event data
// - Add to pending event stream for workflow instance
// - Try to queue workflow task for workflow instance
// - Remove event from future event set and delete event data
//
// KEYS[1] - future event set key
// KEYS[2] - workflow task queue stream
// KEYS[3] - workflow task queue set
// ARGV[1] - current timestamp for zrange
//
// Note: this does not work with Redis Cluster since not all keys are passed into the script.
var futureEventsCmd = redis.NewScript(`
	-- Find events which should become visible now
	local events = redis.call("ZRANGE", KEYS[1], "-inf", ARGV[1], "BYSCORE")
	for i = 1, #events do
		local instanceSegment = redis.call("HGET", events[i], "instance")

		-- Add event to pending event stream
		local eventData = redis.call("HGET", events[i], "event")
		local pending_events_key = "pending-events:" .. instanceSegment
		redis.call("XADD", pending_events_key, "*", "event", eventData)

		-- Try to queue workflow task
		local already_queued = redis.call("SADD", KEYS[3], instanceSegment)
		if already_queued ~= 0 then
			redis.call("XADD", KEYS[2], "*", "id", instanceSegment, "data", "")
		end

		-- Delete event hash data
		redis.call("DEL", events[i])
		redis.call("ZREM", KEYS[1], events[i])
	end

	return #events
`)

func (rb *redisBackend) GetWorkflowTask(ctx context.Context) (*backend.WorkflowTask, error) {
	// Check for future events
	now := time.Now().UnixMilli()
	nowStr := strconv.FormatInt(now, 10)

	queueKeys := rb.workflowQueue.Keys()

	if _, err := futureEventsCmd.Run(ctx, rb.rdb, []string{
		futureEventsKey(),
		queueKeys.StreamKey,
		queueKeys.SetKey,
	}, nowStr).Result(); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("checking future events: %w", err)
	}

	// Try to get a workflow task, this locks the instance when it dequeues one
	instanceTask, err := rb.workflowQueue.Dequeue(ctx, rb.rdb, rb.options.WorkflowLockTimeout, rb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if instanceTask == nil {
		return nil, nil
	}

	instanceState, err := readInstance(ctx, rb.rdb, instanceKeyFromSegment(instanceTask.ID))
	if err != nil {
		return nil, fmt.Errorf("reading workflow instance: %w", err)
	}

	// Read all pending events for this instance
	msgs, err := rb.rdb.XRange(ctx, pendingEventsKey(instanceState.Instance), "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("reading event stream: %w", err)
	}

	payloadKeys := make([]string, 0, len(msgs))
	newEvents := make([]*history.Event, 0, len(msgs))
	for _, msg := range msgs {
		var event *history.Event

		if err := json.Unmarshal([]byte(msg.Values["event"].(string)), &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event: %w", err)
		}

		payloadKeys = append(payloadKeys, payloadKey(event.ID))
		newEvents = append(newEvents, event)
	}

	// Fetch event payloads
	if len(payloadKeys) > 0 {
		res, err := rb.rdb.MGet(ctx, payloadKeys...).Result()
		if err != nil {
			return nil, fmt.Errorf("reading payloads: %w", err)
		}

		for i, event := range newEvents {
			event.Attributes, err = history.DeserializeAttributes(event.Type, []byte(res[i].(string)))
			if err != nil {
				return nil, fmt.Errorf("deserializing attributes for event %v: %w", event.Type, err)
			}
		}
	}

	return &backend.WorkflowTask{
		ID:                    instanceTask.TaskID,
		WorkflowInstance:      instanceState.Instance,
		WorkflowInstanceState: instanceState.State,
		Metadata:              instanceState.Metadata,
		LastSequenceID:        instanceState.LastSequenceID,
		NewEvents:             newEvents,
		CustomData:            msgs[len(msgs)-1].ID, // Id of last pending message in stream at this point
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

// KEYS[1] - pending events
// KEYS[2] - task queue stream
// KEYS[3] - task queue set
// ARGV[1] - Instance segment
var requeueInstanceCmd = redis.NewScript(`
	local pending_events = redis.call("XLEN", KEYS[1])
	if pending_events > 0 then
		local added = redis.call("SADD", KEYS[3], ARGV[1])
		if added == 1 then
			redis.call("XADD", KEYS[2], "*", "id", ARGV[1], "data", "")
		end
	end

	return true
`)

func (rb *redisBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *backend.WorkflowTask,
	instance *core.WorkflowInstance,
	state core.WorkflowInstanceState,
	executedEvents, activityEvents, timerEvents []*history.Event,
	workflowEvents []history.WorkflowEvent,
) error {
	instanceState, err := readInstance(ctx, rb.rdb, instanceKey(instance))
	if err != nil {
		return err
	}

	// Check-point the workflow. We guarantee that no other worker is working on this workflow instance at this point via the
	// task queue, so we don't need to WATCH the keys, we just need to make sure all commands are executed atomically to prevent
	// bad state if a worker crashes in the middle of this execution.
	p := rb.rdb.TxPipeline()

	// Add executed events to the history
	if err := addEventPayloads(ctx, p, executedEvents); err != nil {
		return fmt.Errorf("adding event payloads: %w", err)
	}

	if err := addEventsToStreamP(ctx, p, historyKey(instance), executedEvents); err != nil {
		return fmt.Errorf("serializing : %w", err)
	}

	for _, event := range executedEvents {
		switch event.Type {
		case history.EventType_TimerCanceled:
			removeFutureEventP(ctx, p, instance, event)
		}
	}

	// Schedule timers
	for _, timerEvent := range timerEvents {
		if err := addFutureEventP(ctx, p, instance, timerEvent); err != nil {
			return fmt.Errorf("adding future event: %w", err)
		}
	}

	// Send new workflow events to the respective streams
	groupedEvents := history.EventsByWorkflowInstance(workflowEvents)
	for targetInstance, events := range groupedEvents {
		// Insert pending events for target instance
		for _, m := range events {
			m := m

			if m.HistoryEvent.Type == history.EventType_WorkflowExecutionStarted {
				// Create new instance
				a := m.HistoryEvent.Attributes.(*history.ExecutionStartedAttributes)
				if err := createInstanceP(ctx, p, m.WorkflowInstance, a.Metadata, true); err != nil {
					return err
				}
			}

			// Add pending event to stream
			if err := addEventPayloads(ctx, p, []*history.Event{m.HistoryEvent}); err != nil {
				return fmt.Errorf("adding event payloads: %w", err)
			}

			if err := addEventToStreamP(ctx, p, pendingEventsKey(&targetInstance), m.HistoryEvent); err != nil {
				return fmt.Errorf("adding event to stream: %w", err)
			}
		}

		// Try to enqueue workflow task
		if targetInstance.InstanceID != instance.InstanceID || targetInstance.ExecutionID != instance.ExecutionID {
			if err := rb.workflowQueue.Enqueue(ctx, p, instanceSegment(&targetInstance), nil); err != nil {
				return fmt.Errorf("enqueuing workflow task: %w", err)
			}
		}
	}

	instanceState.State = state

	if state == core.WorkflowInstanceStateFinished || state == core.WorkflowInstanceStateContinuedAsNew {
		t := time.Now()
		instanceState.CompletedAt = &t

		removeActiveInstanceExecutionP(ctx, p, instance)
	}

	if len(executedEvents) > 0 {
		instanceState.LastSequenceID = executedEvents[len(executedEvents)-1].SequenceID
	}

	if err := updateInstanceP(ctx, p, instance, instanceState); err != nil {
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
	if task.CustomData != nil {
		lastPendingEventMessageID := task.CustomData.(string)
		if err := removePendingEventsCmd.Run(ctx, p, []string{pendingEventsKey(instance)}, lastPendingEventMessageID).Err(); err != nil {
			return fmt.Errorf("removing pending events: %w", err)
		}
	}

	// Complete workflow task and unlock instance.
	completeCmd, err := rb.workflowQueue.Complete(ctx, p, task.ID)
	if err != nil {
		return fmt.Errorf("completing workflow task: %w", err)
	}

	// If there are pending events, queue the instance again
	keyInfo := rb.workflowQueue.Keys()
	requeueInstanceCmd.Run(ctx, p,
		[]string{pendingEventsKey(instance), keyInfo.StreamKey, keyInfo.SetKey},
		instanceSegment(instance),
	)

	// Commit transaction
	executedCmds, err := p.Exec(ctx)
	if err != nil {
		if err := completeCmd.Err(); err != nil && err == redis.Nil {
			return fmt.Errorf("could not complete workflow task: %w", err)
		}

		for _, cmd := range executedCmds {
			if cmdErr := cmd.Err(); cmdErr != nil {
				rb.Logger().Debug("redis command error", log.NamespaceKey+".redis.cmd", cmd.Name(), "cmdString", cmd.String(), log.NamespaceKey+".redis.cmdErr", cmdErr.Error())
			}
		}

		return fmt.Errorf("completing workflow task: %w", err)
	}

	if state == core.WorkflowInstanceStateFinished || state == core.WorkflowInstanceStateContinuedAsNew {
		// Trace workflow completion
		ctx, err = (&tracing.TracingContextPropagator{}).Extract(ctx, instanceState.Metadata)
		if err != nil {
			rb.Logger().Error("extracting tracing context", log.ErrorKey, err)
		}

		_, span := rb.Tracer().Start(ctx, "WorkflowComplete",
			trace.WithAttributes(
				attribute.String(log.NamespaceKey+log.InstanceIDKey, instanceState.Instance.InstanceID),
			))
		span.End()

		if rb.options.AutoExpiration > 0 {
			if err := setWorkflowInstanceExpiration(ctx, rb.rdb, instance, rb.options.AutoExpiration); err != nil {
				return fmt.Errorf("setting workflow instance expiration: %w", err)
			}
		}
	}

	return nil
}

func (rb *redisBackend) addWorkflowInstanceEventP(ctx context.Context, p redis.Pipeliner, instance *core.WorkflowInstance, event *history.Event) error {
	// Add event to pending events for instance
	if err := addEventPayloads(ctx, p, []*history.Event{event}); err != nil {
		return err
	}

	if err := addEventToStreamP(ctx, p, pendingEventsKey(instance), event); err != nil {
		return err
	}

	// Queue workflow task
	if err := rb.workflowQueue.Enqueue(ctx, p, instanceSegment(instance), nil); err != nil {
		return fmt.Errorf("queueing workflow: %w", err)
	}

	return nil
}
