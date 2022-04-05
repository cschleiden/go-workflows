package redis

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
	"github.com/cschleiden/go-workflows/internal/task"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

func NewRedisBackend(address, username, password string, db int, opts ...backend.BackendOption) backend.Backend {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{address},
		Username: username,
		Password: password,
		DB:       db,
	})

	// TODO: Only for dev
	client.FlushDB(context.Background())

	return &redisBackend{
		rdb:     client,
		options: backend.ApplyOptions(opts...),
	}
}

type redisBackend struct {
	rdb     redis.UniversalClient
	options backend.Options
}

func (rb *redisBackend) CreateWorkflowInstance(ctx context.Context, event history.WorkflowEvent) error {
	// Store instance with its state
	if err := createInstance(ctx, rb.rdb, event.WorkflowInstance, &instanceState{
		InstanceID:  event.WorkflowInstance.GetInstanceID(),
		ExecutionID: event.WorkflowInstance.GetExecutionID(),
		State:       backend.WorkflowStateActive,
		CreatedAt:   time.Now(),
	}); err != nil {
		return errors.Wrap(err, "could not create workflow instance")
	}

	// Create event stream
	eventData, err := json.Marshal(event.HistoryEvent)
	if err != nil {
		return err
	}

	cmd := rb.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: pendingEventsKey(event.WorkflowInstance.GetInstanceID()),
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	})
	_, err = cmd.Result()
	if err != nil {
		return errors.Wrap(err, "could not create event stream")
	}

	// Add instance to pending instances set
	zcmd := rb.rdb.ZAdd(ctx, pendingInstancesKey(), &redis.Z{Score: float64(0), Member: event.WorkflowInstance.GetInstanceID()})
	if err := zcmd.Err(); err != nil {
		return errors.Wrap(err, "could not add instance to locked instances set")
	}

	return nil
}

func (rb *redisBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	var instanceID string

	getAndLockTask := func(tx *redis.Tx) error {
		instanceID = ""

		// Find pending workflow instance from sorted set
		now := int(time.Now().Unix())
		cmd := tx.ZRangeByScoreWithScores(ctx, pendingInstancesKey(), &redis.ZRangeBy{
			// Get at most one task
			Count: 1,
			// Unlocked tasks have a score of 0 so start at -inf
			Min: "-inf",
			// Abandoned tasks will have an unlock-timestap in the past, so include those as well
			Max: strconv.Itoa(now),
		})
		if err := cmd.Err(); err != nil {
			return errors.Wrap(err, "could not get pending instance")
		}

		r, err := cmd.Result()
		if err != nil {
			return errors.Wrap(err, "could not get pending instance")
		}

		if len(r) == 0 {
			return nil
		}

		// log.Println("Member ", r[0].Member, "Score", r[0].Score)
		instanceID = r[0].Member.(string)

		// Mark instance as locked
		lockedUntil := time.Now().Add(rb.options.WorkflowLockTimeout)

		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			// Overwrite the key with the new score
			p.ZAdd(ctx, pendingInstancesKey(), &redis.Z{Score: float64(lockedUntil.Unix()), Member: instanceID})
			return nil
		})

		return err
	}

	for i := 0; i < 10; i++ {
		err := rb.rdb.Watch(ctx, getAndLockTask, pendingInstancesKey())
		if err == nil {
			// Success.
			break
		}

		if err == redis.TxFailedErr {
			// Optimistic lock lost. Retry.
			continue
		}

		// Return any other error.
		return nil, errors.Wrap(err, "could not find workflow task")
	}

	if instanceID == "" {
		return nil, nil
	}

	instanceState, err := readInstance(ctx, rb.rdb, instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "could not read workflow instance")
	}

	// Read stream

	// History
	cmd := rb.rdb.XRange(ctx, eventsKey(instanceID), "-", "+")
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

	cmd = rb.rdb.XRange(ctx, pendingEventsKey(instanceID), "-", "+")
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
	rb.rdb.XTrim(ctx, pendingEventsKey(instanceID), 0)

	log.Println("Returned task for ", instanceID)

	return &task.Workflow{
		WorkflowInstance: core.NewWorkflowInstance(instanceID, instanceState.ExecutionID),
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
			Stream: eventsKey(instance.GetInstanceID()),
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
			zcmd := rb.rdb.ZAdd(ctx, pendingInstancesKey(), &redis.Z{Score: float64(0), Member: targetInstance.GetInstanceID()})
			if err := zcmd.Err(); err != nil {
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

		// Add task to queue
		rb.rdb.ZAdd(ctx, activitiesKey(), &redis.Z{Score: float64(0), Member: activityEvent.ID})
	}

	// Unlock instance
	cmd := rb.rdb.ZRem(ctx, pendingInstancesKey(), instance.GetInstanceID())
	if removed, err := cmd.Result(); err != nil {
		return errors.Wrap(err, "could not remove instance from locked instances set")
	} else if removed == 0 {
		return errors.Wrap(err, "instance already unlocked")
	}

	log.Println("Unlocked", instance.GetInstanceID())

	return nil
}

func (rb *redisBackend) CancelWorkflowInstance(ctx context.Context, instance core.WorkflowInstance) error {
	panic("unimplemented")
}

func (rb *redisBackend) SignalWorkflow(ctx context.Context, instanceID string, event history.Event) error {
	// TODO: Store signal event

	panic("unimplemented")
}

func (rb *redisBackend) GetWorkflowInstanceHistory(ctx context.Context, instance core.WorkflowInstance) ([]history.Event, error) {
	panic("unimplemented")
}

func (rb *redisBackend) GetWorkflowInstanceState(ctx context.Context, instance core.WorkflowInstance) (backend.WorkflowState, error) {
	// panic("unimplemented")
	// TODO: Implement
	return backend.WorkflowStateActive, nil
}

func (rb *redisBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	var activityID string

	getAndLockTask := func(tx *redis.Tx) error {
		activityID = ""

		// Find pending workflow instance from sorted set
		now := int(time.Now().Unix())
		cmd := tx.ZRangeByScoreWithScores(ctx, activitiesKey(), &redis.ZRangeBy{
			// Get at most one task
			Count: 1,
			// Unlocked tasks have a score of 0 so start at -inf
			Min: "-inf",
			// Abandoned tasks will have an unlock-timestap in the past, so include those as well
			Max: strconv.Itoa(now),
		})
		if err := cmd.Err(); err != nil {
			return errors.Wrap(err, "could not get an activity task")
		}

		r, err := cmd.Result()
		if err != nil {
			return errors.Wrap(err, "could not get an activity task")
		}

		if len(r) == 0 {
			return nil
		}

		activityID = r[0].Member.(string)

		// Mark instance as locked
		lockedUntil := time.Now().Add(rb.options.ActivityLockTimeout)

		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			// Overwrite the key with the new score
			p.ZAdd(ctx, activitiesKey(), &redis.Z{Score: float64(lockedUntil.Unix()), Member: activityID})
			return nil
		})

		return err
	}

	for i := 0; i < 10; i++ {
		err := rb.rdb.Watch(ctx, getAndLockTask, activitiesKey())
		if err == nil {
			// Success.
			break
		}

		if err == redis.TxFailedErr {
			// Optimistic lock lost. Retry.
			continue
		}

		// Return any other error.
		return nil, errors.Wrap(err, "could not find workflow task")
	}

	if activityID == "" {
		return nil, nil
	}

	activity, err := getActivity(ctx, rb.rdb, activityID)
	if err != nil {
		return nil, err
	}

	return &task.Activity{
		// TODO: Include execution id
		WorkflowInstance: core.NewWorkflowInstance(activity.InstanceID, ""),
		ID:               activity.ID,
		Event:            activity.Event,
	}, nil
}

func (rb *redisBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	panic("unimplemented")
}

func (rb *redisBackend) CompleteActivityTask(ctx context.Context, instance core.WorkflowInstance, activityID string, event history.Event) error {
	// Deliver event to workflow instance
	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	cmd := rb.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: pendingEventsKey(instance.GetInstanceID()),
		ID:     "*",
		Values: map[string]interface{}{
			"event": string(eventData),
		},
	})
	_, err = cmd.Result()
	if err != nil {
		return errors.Wrap(err, "could not create event stream")
	}

	zcmd := rb.rdb.ZAdd(ctx, pendingInstancesKey(), &redis.Z{Score: float64(0), Member: instance.GetInstanceID()})
	if err := zcmd.Err(); err != nil {
		return errors.Wrap(err, "could not add instance to locked instances set")
	}

	// Unlock activity
	rcmd := rb.rdb.ZRem(ctx, activitiesKey(), activityID)
	if removed, err := rcmd.Result(); err != nil {
		return errors.Wrap(err, "could not remove activity from locked activities set")
	} else if removed == 0 {
		return errors.Wrap(err, "activity already unlocked")
	}

	// TODO: Remove state

	return nil
}
