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
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (rb *redisBackend) GetWorkflowTask(ctx context.Context, queues []workflow.Queue) (*backend.WorkflowTask, error) {
	if err := scheduleFutureEvents(ctx, rb); err != nil {
		return nil, fmt.Errorf("scheduling future events: %w", err)
	}

	// Try to get a workflow task, this locks the instance when it dequeues one
	instanceTask, err := rb.workflowQueue.Dequeue(ctx, rb.rdb, queues, rb.options.WorkflowLockTimeout, rb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if instanceTask == nil {
		return nil, nil
	}

	instanceState, err := readInstance(ctx, rb.rdb, rb.keys.instanceKeyFromSegment(instanceTask.ID))
	if err != nil {
		return nil, fmt.Errorf("reading workflow instance: %w", err)
	}

	// Read all pending events for this instance
	msgs, err := rb.rdb.XRange(ctx, rb.keys.pendingEventsKey(instanceState.Instance), "-", "+").Result()
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

		payloadKeys = append(payloadKeys, event.ID)
		newEvents = append(newEvents, event)
	}

	// Fetch event payloads
	if len(payloadKeys) > 0 {
		res, err := rb.rdb.HMGet(ctx, rb.keys.payloadKey(instanceState.Instance), payloadKeys...).Result()
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

func (rb *redisBackend) ExtendWorkflowTask(ctx context.Context, task *backend.WorkflowTask) error {
	_, err := rb.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		return rb.workflowQueue.Extend(ctx, p, task.Queue, task.ID)
	})

	return err
}

func (rb *redisBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *backend.WorkflowTask,
	state core.WorkflowInstanceState,
	executedEvents, activityEvents, timerEvents []*history.Event,
	workflowEvents []*history.WorkflowEvent,
) error {
	keys := make([]string, 0)
	args := make([]interface{}, 0)

	instance := task.WorkflowInstance

	queueKeys := rb.workflowQueue.Keys(task.Queue)
	keys = append(keys,
		rb.keys.instanceKey(instance),
		rb.keys.historyKey(instance),
		rb.keys.pendingEventsKey(instance),
		rb.keys.payloadKey(instance),
		rb.keys.futureEventsKey(),
		rb.keys.instancesActive(),
		rb.keys.instancesByCreation(),
		queueKeys.SetKey,
		queueKeys.StreamKey,
		rb.workflowQueue.queueSetKey,
	)
	args = append(args, rb.keys.prefix, instanceSegment(instance))

	// Add executed events to the history
	args = append(args, len(executedEvents))

	for _, event := range executedEvents {
		eventData, err := marshalEventWithoutAttributes(event)
		if err != nil {
			return fmt.Errorf("marshaling event: %w", err)
		}

		payloadData, err := json.Marshal(event.Attributes)
		if err != nil {
			return fmt.Errorf("marshaling event payload: %w", err)
		}

		args = append(args, event.ID, eventData, payloadData, event.SequenceID)
	}

	// Remove executed pending events
	lastPendingEventMessageID := task.CustomData.(string)
	args = append(args, lastPendingEventMessageID)

	// Update instance state and update active execution
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	nowUnix := now.Unix()
	args = append(
		args,
		string(nowStr),
		nowUnix,
		int(state),
		int(core.WorkflowInstanceStateContinuedAsNew),
		int(core.WorkflowInstanceStateFinished),
	)
	keys = append(keys, rb.keys.activeInstanceExecutionKey(instance.InstanceID))

	// Remove canceled timers
	timersToCancel := make([]*history.Event, 0)
	for _, event := range executedEvents {
		switch event.Type {
		case history.EventType_TimerCanceled:
			timersToCancel = append(timersToCancel, event)
		}
	}

	args = append(args, len(timersToCancel))
	for _, event := range timersToCancel {
		keys = append(keys, rb.keys.futureEventKey(instance, event.ScheduleEventID))
	}

	// Schedule timers
	args = append(args, len(timerEvents))
	for _, timerEvent := range timerEvents {
		eventData, err := marshalEventWithoutAttributes(timerEvent)
		if err != nil {
			return fmt.Errorf("marshaling event: %w", err)
		}

		payloadEventData, err := json.Marshal(timerEvent.Attributes)
		if err != nil {
			return fmt.Errorf("marshaling event payload: %w", err)
		}

		args = append(args, timerEvent.ID, strconv.FormatInt(timerEvent.VisibleAt.UnixMilli(), 10), eventData, payloadEventData)
		keys = append(keys, rb.keys.futureEventKey(instance, timerEvent.ScheduleEventID))
	}

	// Schedule activities
	args = append(args, len(activityEvents))
	for _, activityEvent := range activityEvents {
		activityData, err := json.Marshal(&activityData{
			Instance: instance,
			ID:       activityEvent.ID,
			Event:    activityEvent,
		})
		if err != nil {
			return fmt.Errorf("marshaling activity data: %w", err)
		}

		activityQueue := string(task.Queue) // TODO: support sending activities to custom queues
		args = append(args, activityQueue, activityEvent.ID, activityData)
	}

	// Send new workflow events to the respective streams
	groupedEvents := history.EventsByWorkflowInstance(workflowEvents)
	args = append(args, len(groupedEvents))
	for targetInstance, events := range groupedEvents {
		keys = append(keys, rb.keys.instanceKey(&targetInstance), rb.keys.activeInstanceExecutionKey(targetInstance.InstanceID))
		args = append(args, instanceSegment(&targetInstance), targetInstance.InstanceID)

		// Are we creating a new workflow instance?
		m := events[0]
		createNewInstance := m.HistoryEvent.Type == history.EventType_WorkflowExecutionStarted
		args = append(args, createNewInstance)
		args = append(args, len(events))

		if createNewInstance {
			a := m.HistoryEvent.Attributes.(*history.ExecutionStartedAttributes)
			isb, err := json.Marshal(&instanceState{
				Queue:     string(task.Queue),
				Instance:  &targetInstance,
				State:     core.WorkflowInstanceStateActive,
				Metadata:  a.Metadata,
				CreatedAt: time.Now(),
			})
			if err != nil {
				return fmt.Errorf("marshaling new instance state: %w", err)
			}

			ib, err := json.Marshal(targetInstance)
			if err != nil {
				return fmt.Errorf("marshaling instance: %w", err)
			}

			args = append(args, isb, ib)

			// Create pending event for conflicts
			pfe := history.NewPendingEvent(time.Now(), history.EventType_SubWorkflowFailed, &history.SubWorkflowFailedAttributes{
				Error: workflowerrors.FromError(backend.ErrInstanceAlreadyExists),
			}, history.ScheduleEventID(m.WorkflowInstance.ParentEventID))
			eventData, payloadEventData, err := marshalEvent(pfe)
			if err != nil {
				return fmt.Errorf("marshaling event: %w", err)
			}

			args = append(args, pfe.ID, eventData, payloadEventData)
		}

		keys = append(keys, rb.keys.pendingEventsKey(&targetInstance), rb.keys.payloadKey(&targetInstance))
		for _, m := range events {
			eventData, payloadEventData, err := marshalEvent(m.HistoryEvent)
			if err != nil {
				return fmt.Errorf("marshaling event: %w", err)
			}

			args = append(args, m.HistoryEvent.ID, eventData, payloadEventData)
		}
	}

	// Complete workflow task and unlock instance.
	args = append(args, task.ID, rb.workflowQueue.groupName)

	// If there are pending events, queue the instance again
	// 	No args/keys needed

	// Run script
	_, err := completeWorkflowTaskCmd.Run(ctx, rb.rdb, keys, args...).Result()
	if err != nil {
		return fmt.Errorf("completing workflow task: %w", err)
	}

	if state == core.WorkflowInstanceStateFinished || state == core.WorkflowInstanceStateContinuedAsNew {
		// Trace workflow completion
		ctx, err = (&tracing.TracingContextPropagator{}).Extract(ctx, task.Metadata)
		if err != nil {
			rb.options.Logger.Error("extracting tracing context", log.ErrorKey, err)
		}

		_, span := rb.Tracer().Start(ctx, "WorkflowComplete",
			trace.WithAttributes(
				attribute.String(log.NamespaceKey+log.InstanceIDKey, task.WorkflowInstance.InstanceID),
			))
		span.End()

		// Auto expiration
		expiration := rb.options.AutoExpiration
		if state == core.WorkflowInstanceStateContinuedAsNew && rb.options.AutoExpirationContinueAsNew > 0 {
			expiration = rb.options.AutoExpirationContinueAsNew
		}

		if expiration > 0 {
			if err := rb.setWorkflowInstanceExpiration(ctx, instance, expiration); err != nil {
				return fmt.Errorf("setting workflow instance expiration: %w", err)
			}
		}
	}

	return nil
}

func marshalEvent(event *history.Event) (string, string, error) {
	eventData, err := marshalEventWithoutAttributes(event)
	if err != nil {
		return "", "", fmt.Errorf("marshaling event payload: %w", err)
	}

	payloadEventData, err := json.Marshal(event.Attributes)
	if err != nil {
		return "", "", fmt.Errorf("marshaling event payload: %w", err)
	}
	return eventData, string(payloadEventData), nil
}

func (rb *redisBackend) addWorkflowInstanceEventP(ctx context.Context, p redis.Pipeliner, queue workflow.Queue, instance *core.WorkflowInstance, event *history.Event) error {
	// Add event to pending events for instance
	if err := rb.addEventPayloadsP(ctx, p, instance, []*history.Event{event}); err != nil {
		return err
	}

	if err := addEventToStreamP(ctx, p, rb.keys.pendingEventsKey(instance), event); err != nil {
		return err
	}

	// Queue workflow task
	if err := rb.workflowQueue.Enqueue(ctx, p, queue, instanceSegment(instance), nil); err != nil {
		return fmt.Errorf("queueing workflow: %w", err)
	}

	return nil
}
