package valkey

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
	"github.com/cschleiden/go-workflows/internal/propagators"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/valkey-io/valkey-glide/go/v2/options"
)

func (vb *valkeyBackend) PrepareWorkflowQueues(ctx context.Context, queues []workflow.Queue) error {
	return vb.workflowQueue.Prepare(ctx, vb.client, queues)
}

func (vb *valkeyBackend) GetWorkflowTask(ctx context.Context, queues []workflow.Queue) (*backend.WorkflowTask, error) {
	if err := scheduleFutureEvents(ctx, vb); err != nil {
		return nil, fmt.Errorf("scheduling future events: %w", err)
	}

	// Try to get a workflow task, this locks the instance when it dequeues one
	instanceTask, err := vb.workflowQueue.Dequeue(ctx, vb.client, queues, vb.options.WorkflowLockTimeout, vb.options.BlockTimeout)
	if err != nil {
		return nil, err
	}

	if instanceTask == nil {
		return nil, nil
	}

	instanceState, err := readInstance(ctx, vb.client, vb.keys.instanceKeyFromSegment(instanceTask.ID))
	if err != nil {
		return nil, fmt.Errorf("reading workflow instance: %w", err)
	}

	// Read all pending events for this instance
	msgs, err := vb.client.XRange(ctx, vb.keys.pendingEventsKey(instanceState.Instance), "-", "+")
	if err != nil {
		return nil, fmt.Errorf("reading event stream: %w", err)
	}

	payloadKeys := make([]string, 0, len(msgs))
	newEvents := make([]*history.Event, 0, len(msgs))
	for _, msg := range msgs {
		var eventStr string
		for _, field := range msg.Fields {
			if field.Field == "event" {
				eventStr = field.Value
				break
			}
		}

		var event *history.Event
		if err := json.Unmarshal([]byte(eventStr), &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event: %w", err)
		}

		payloadKeys = append(payloadKeys, event.ID)
		newEvents = append(newEvents, event)
	}

	// Fetch event payloads
	if len(payloadKeys) > 0 {
		res, err := vb.client.HMGet(ctx, vb.keys.payloadKey(instanceState.Instance), payloadKeys)
		if err != nil {
			return nil, fmt.Errorf("reading payloads: %w", err)
		}

		for i, event := range newEvents {
			event.Attributes, err = history.DeserializeAttributes(event.Type, []byte(res[i].Value()))
			if err != nil {
				return nil, fmt.Errorf("deserializing attributes for event %v: %w", event.Type, err)
			}
		}
	}

	return &backend.WorkflowTask{
		ID:                    instanceTask.TaskID,
		Queue:                 core.Queue(instanceState.Queue),
		WorkflowInstance:      instanceState.Instance,
		WorkflowInstanceState: instanceState.State,
		Metadata:              instanceState.Metadata,
		LastSequenceID:        instanceState.LastSequenceID,
		NewEvents:             newEvents,
		CustomData:            msgs[len(msgs)-1].ID,
	}, nil
}

func (vb *valkeyBackend) ExtendWorkflowTask(ctx context.Context, task *backend.WorkflowTask) error {
	return vb.workflowQueue.Extend(ctx, vb.client, task.Queue, task.ID)
}

func (vb *valkeyBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *backend.WorkflowTask,
	state core.WorkflowInstanceState,
	executedEvents, activityEvents, timerEvents []*history.Event,
	workflowEvents []*history.WorkflowEvent,
) error {
	keys := make([]string, 0)
	args := make([]string, 0)

	instance := task.WorkflowInstance

	queueKeys := vb.workflowQueue.Keys(task.Queue)
	keys = append(keys,
		vb.keys.instanceKey(instance),
		vb.keys.historyKey(instance),
		vb.keys.pendingEventsKey(instance),
		vb.keys.payloadKey(instance),
		vb.keys.futureEventsKey(),
		vb.keys.instancesActive(),
		vb.keys.instancesByCreation(),
		queueKeys.SetKey,
		queueKeys.StreamKey,
		vb.workflowQueue.queueSetKey,
	)
	args = append(args, vb.keys.prefix, instanceSegment(instance))

	// Add executed events to the history
	args = append(args, fmt.Sprintf("%d", len(executedEvents)))

	for _, event := range executedEvents {
		eventData, payloadData, err := marshalEvent(event)
		if err != nil {
			return err
		}

		args = append(args, event.ID, eventData, payloadData, fmt.Sprintf("%d", event.SequenceID))
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
		nowStr,
		fmt.Sprintf("%d", nowUnix),
		fmt.Sprintf("%d", int(state)),
		fmt.Sprintf("%d", int(core.WorkflowInstanceStateContinuedAsNew)),
		fmt.Sprintf("%d", int(core.WorkflowInstanceStateFinished)),
	)
	keys = append(keys, vb.keys.activeInstanceExecutionKey(instance.InstanceID))

	// Remove canceled timers
	timersToCancel := make([]*history.Event, 0)
	for _, event := range executedEvents {
		switch event.Type {
		case history.EventType_TimerCanceled:
			timersToCancel = append(timersToCancel, event)
		default:
			return fmt.Errorf("unexpected event type %v", event.Type)
		}
	}

	args = append(args, fmt.Sprintf("%d", len(timersToCancel)))
	for _, event := range timersToCancel {
		keys = append(keys, vb.keys.futureEventKey(instance, event.ScheduleEventID))
	}

	// Schedule timers
	args = append(args, fmt.Sprintf("%d", len(timerEvents)))
	for _, timerEvent := range timerEvents {
		eventData, payloadEventData, err := marshalEvent(timerEvent)
		if err != nil {
			return err
		}

		args = append(args, timerEvent.ID, strconv.FormatInt(timerEvent.VisibleAt.UnixMilli(), 10), eventData, payloadEventData)
		keys = append(keys, vb.keys.futureEventKey(instance, timerEvent.ScheduleEventID))
	}

	// Schedule activities
	args = append(args, fmt.Sprintf("%d", len(activityEvents)))
	for _, activityEvent := range activityEvents {
		a := activityEvent.Attributes.(*history.ActivityScheduledAttributes)
		queue := a.Queue
		if queue == "" {
			// Default to workflow queue
			queue = task.Queue
		}

		activityData, err := json.Marshal(&activityData{
			Instance: instance,
			ID:       activityEvent.ID,
			Event:    activityEvent,
			Queue:    string(queue),
		})
		if err != nil {
			return fmt.Errorf("marshaling activity data: %w", err)
		}

		activityQueue := string(queue)
		args = append(args, activityQueue, activityEvent.ID, string(activityData))
	}

	// Send new workflow events to the respective streams
	groupedEvents := history.EventsByWorkflowInstance(workflowEvents)
	args = append(args, fmt.Sprintf("%d", len(groupedEvents)))
	for targetInstance, events := range groupedEvents {
		keys = append(keys, vb.keys.instanceKey(&targetInstance), vb.keys.activeInstanceExecutionKey(targetInstance.InstanceID))
		args = append(args, instanceSegment(&targetInstance), targetInstance.InstanceID)

		// Are we creating a new workflow instance?
		m := events[0]
		createNewInstance := m.HistoryEvent.Type == history.EventType_WorkflowExecutionStarted
		args = append(args, fmt.Sprintf("%v", createNewInstance))
		args = append(args, fmt.Sprintf("%d", len(events)))

		if createNewInstance {
			a := m.HistoryEvent.Attributes.(*history.ExecutionStartedAttributes)

			queue := a.Queue
			if queue == "" {
				queue = task.Queue
			}

			isb, err := json.Marshal(&instanceState{
				Queue:     string(queue),
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

			args = append(args, string(isb), string(ib))

			// Create pending event for conflicts
			pfe := history.NewPendingEvent(time.Now(), history.EventType_SubWorkflowFailed, &history.SubWorkflowFailedAttributes{
				Error: workflowerrors.FromError(backend.ErrInstanceAlreadyExists),
			}, history.ScheduleEventID(m.WorkflowInstance.ParentEventID))
			eventData, payloadEventData, err := marshalEvent(pfe)
			if err != nil {
				return fmt.Errorf("marshaling event: %w", err)
			}

			args = append(args, pfe.ID, eventData, payloadEventData)

			queueKeys := vb.workflowQueue.Keys(queue)
			keys = append(keys, queueKeys.SetKey, queueKeys.StreamKey)
		} else {
			targetInstanceState, err := readInstance(ctx, vb.client, vb.keys.instanceKey(&targetInstance))
			if err != nil {
				return fmt.Errorf("reading target instance: %w", err)
			}

			queueKeys := vb.workflowQueue.Keys(core.Queue(targetInstanceState.Queue))
			keys = append(keys, queueKeys.SetKey, queueKeys.StreamKey)
		}

		keys = append(keys, vb.keys.pendingEventsKey(&targetInstance), vb.keys.payloadKey(&targetInstance))
		for _, m := range events {
			eventData, payloadEventData, err := marshalEvent(m.HistoryEvent)
			if err != nil {
				return fmt.Errorf("marshaling event: %w", err)
			}

			args = append(args, m.HistoryEvent.ID, eventData, payloadEventData)
		}
	}

	// Complete workflow task and unlock instance.
	args = append(args, task.ID, vb.workflowQueue.groupName)

	// Run script
	_, err := vb.client.InvokeScriptWithOptions(ctx, completeWorkflowTaskScript, options.ScriptOptions{
		Keys: keys,
		Args: args,
	})
	if err != nil {
		return fmt.Errorf("completing workflow task: %w", err)
	}

	if state == core.WorkflowInstanceStateFinished || state == core.WorkflowInstanceStateContinuedAsNew {
		// Trace workflow completion
		ctx, err := (&propagators.TracingContextPropagator{}).Extract(ctx, task.Metadata)
		if err != nil {
			vb.options.Logger.Error("extracting tracing context", log.ErrorKey, err)
		}

		// Auto expiration
		expiration := vb.options.AutoExpiration
		if state == core.WorkflowInstanceStateContinuedAsNew && vb.options.AutoExpirationContinueAsNew > 0 {
			expiration = vb.options.AutoExpirationContinueAsNew
		}

		if expiration > 0 {
			if err := vb.setWorkflowInstanceExpiration(ctx, instance, expiration); err != nil {
				return fmt.Errorf("setting workflow instance expiration: %w", err)
			}
		}

		if vb.options.RemoveContinuedAsNewInstances && state == core.WorkflowInstanceStateContinuedAsNew {
			if err := vb.RemoveWorkflowInstance(ctx, instance); err != nil {
				return fmt.Errorf("removing workflow instance: %w", err)
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
