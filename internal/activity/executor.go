package activity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/payload"
	"github.com/cschleiden/go-workflows/internal/args"
	"github.com/cschleiden/go-workflows/internal/log"
	"github.com/cschleiden/go-workflows/internal/tracing"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/registry"
	wf "github.com/cschleiden/go-workflows/workflow"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Executor struct {
	logger      *slog.Logger
	tracer      trace.Tracer
	converter   converter.Converter
	propagators []wf.ContextPropagator
	r           *registry.Registry
}

func NewExecutor(
	logger *slog.Logger,
	tracer trace.Tracer,
	converter converter.Converter,
	propagators []wf.ContextPropagator,
	r *registry.Registry,
) *Executor {
	return &Executor{
		logger:      logger,
		tracer:      tracer,
		converter:   converter,
		propagators: propagators,
		r:           r,
	}
}

func (e *Executor) ExecuteActivity(ctx context.Context, task *backend.ActivityTask) (payload.Payload, error) {
	a := task.Event.Attributes.(*history.ActivityScheduledAttributes)

	// Add activity state to context
	as := NewActivityState(
		task.Event.ID,
		a.Attempt,
		task.WorkflowInstance,
		e.logger)
	activityCtx := WithActivityState(ctx, as)

	for _, propagator := range e.propagators {
		var err error
		activityCtx, err = propagator.Extract(activityCtx, a.Metadata)
		if err != nil {
			return nil, workflowerrors.NewPermanentError(fmt.Errorf("extracting context from propagator: %w", err))
		}
	}

	activityCtx, span := e.tracer.Start(activityCtx, fmt.Sprintf("ActivityTaskExecution: %s", a.Name), trace.WithAttributes(
		attribute.String(log.ActivityNameKey, a.Name),
		attribute.String(log.InstanceIDKey, task.WorkflowInstance.InstanceID),
		attribute.String(log.ActivityIDKey, task.ID),
		attribute.Int(log.AttemptKey, a.Attempt),
	))
	defer span.End()

	activity, err := e.r.GetActivity(a.Name)
	if err != nil {
		return nil, workflowerrors.NewPermanentError(tracing.WithSpanError(span, fmt.Errorf("activity not found: %w", err)))
	}

	activityFn := reflect.ValueOf(activity)
	if activityFn.Type().Kind() != reflect.Func {
		return nil, workflowerrors.NewPermanentError(tracing.WithSpanError(span, errors.New("activity not a function")))
	}

	args, addContext, err := args.InputsToArgs(e.converter, activityFn, a.Inputs)
	if err != nil {
		return nil, workflowerrors.NewPermanentError(tracing.WithSpanError(span, fmt.Errorf("converting activity inputs: %w", err)))
	}

	// Execute activity
	if addContext {
		args[0] = reflect.ValueOf(activityCtx)
	}

	done := make(chan struct{})
	var rv []reflect.Value

	go func() {
		// Recover any panic encountered during activity execution
		defer func() {
			if r := recover(); r != nil {
				err := workflowerrors.NewPanicError(fmt.Sprintf("panic: %v", r))
				rv = []reflect.Value{reflect.ValueOf(err)}
			}

			close(done)
		}()

		rv = activityFn.Call(args)
	}()

	<-done

	if len(rv) < 1 || len(rv) > 2 {
		return nil, workflowerrors.NewPermanentError(
			tracing.WithSpanError(span, errors.New("activity has to return either (error) or (<result>, error)")))
	}

	var result payload.Payload

	// Convert activity result to payload. We always expect at least an error
	if len(rv) > 1 {
		var err error
		result, err = e.converter.To(rv[0].Interface())
		if err != nil {
			return nil, workflowerrors.NewPermanentError(tracing.WithSpanError(span, fmt.Errorf("converting activity result: %w", err)))
		}
	}

	// Was an error returned?
	errResult := rv[len(rv)-1]
	if errResult.IsNil() {
		// No error from activity execution
		return result, nil
	}

	err, ok := errResult.Interface().(error)
	if !ok {
		return nil, workflowerrors.NewPermanentError(
			tracing.WithSpanError(span, fmt.Errorf("activity error result does not satisfy error interface (%T): %v", errResult, errResult)))
	}

	return result, workflowerrors.FromError(tracing.WithSpanError(span, err))
}
