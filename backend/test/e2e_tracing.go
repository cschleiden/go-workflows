package test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/registry"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
)

func setupTracing(b TestBackend) *tracetest.InMemoryExporter {
	exporter := tracetest.NewInMemoryExporter()
	processor := trace.NewSimpleSpanProcessor(exporter)
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(processor))
	b.Options().TracerProvider = provider

	return exporter
}

var e2eTracingTests = []backendTest{
	{
		name: "Tracing/WorkflowsHaveSpans",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			exporter := setupTracing(b)

			wf := func(ctx workflow.Context) error {
				return nil
			}
			register(t, ctx, w, []interface{}{wf}, nil)

			instance := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)

			spans := exporter.GetSpans().Snapshots()

			createWorkflowSpan := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "CreateWorkflowInstance")
			})
			require.NotNil(t, createWorkflowSpan)

			workflow1Span := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Workflow: 1")
			})
			require.NotNil(t, workflow1Span)

			require.Equal(t,
				createWorkflowSpan.SpanContext().SpanID().String(),
				workflow1Span.Parent().SpanID().String(),
			)
		},
	},
	{
		name: "Tracing/TimersHaveSpans",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			exporter := setupTracing(b)

			wf := func(ctx workflow.Context) error {
				workflow.ScheduleTimer(ctx, time.Millisecond*20).Get(ctx)

				workflow.Sleep(ctx, time.Millisecond*10)

				return nil
			}
			register(t, ctx, w, []interface{}{wf}, nil)

			instance := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)

			spans := exporter.GetSpans().Snapshots()

			workflow1Span := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Workflow: 1")
			})
			require.NotNil(t, workflow1Span)

			timerSpan := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Timer")
			})
			require.NotNil(t, workflow1Span)
			require.InEpsilon(t, time.Duration(20*time.Millisecond),
				timerSpan.EndTime().Sub(timerSpan.StartTime())/time.Millisecond,
				float64(5*time.Millisecond))
			require.Equal(t, "Timer", timerSpan.Name())

			sleepSpan := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Timer: Sleep")
			})
			require.NotNil(t, sleepSpan)

			require.Equal(t,
				workflow1Span.SpanContext().SpanID().String(),
				timerSpan.Parent().SpanID().String(),
			)
		},
	},
	{
		name: "Tracing/TimersWithinCustomSpans",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			exporter := setupTracing(b)

			wf := func(ctx workflow.Context) error {
				ctx, span := workflow.Tracer(ctx).Start(ctx, "custom-span")
				defer span.End()

				workflow.Sleep(ctx, time.Millisecond*10)

				return nil
			}
			register(t, ctx, w, []interface{}{wf}, nil)

			instance := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)

			spans := exporter.GetSpans().Snapshots()

			workflow1Span := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Workflow: 1")
			})
			require.NotNil(t, workflow1Span)

			customSpan := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "custom-span")
			})
			require.NotNil(t, workflow1Span)
			require.Equal(t,
				workflow1Span.SpanContext().SpanID().String(),
				customSpan.Parent().SpanID().String(),
			)

			sleepSpan := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Timer: Sleep")
			})
			require.NotNil(t, sleepSpan)
			require.Equal(t,
				customSpan.SpanContext().SpanID().String(),
				sleepSpan.Parent().SpanID().String(),
			)
		},
	},
	{
		name: "Tracing/SubWorkflowsHaveSpansWithCorrectParent",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			exporter := setupTracing(b)

			swf := func(ctx workflow.Context) error {
				workflow.Sleep(ctx, time.Microsecond*10)

				return nil
			}

			wf := func(ctx workflow.Context) error {
				_, err := workflow.CreateSubWorkflowInstance[any](ctx, workflow.DefaultSubWorkflowOptions, "swf").Get(ctx)
				return err
			}

			w.RegisterWorkflow(wf, registry.WithName("wf"))
			w.RegisterWorkflow(swf, registry.WithName("swf"))

			err := w.Start(ctx)
			require.NoError(t, err)

			instance := runWorkflow(t, ctx, c, "wf")
			_, err = client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)

			spans := exporter.GetSpans().Snapshots()

			createWorkflowSpan := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "CreateWorkflowInstance")
			})
			require.NotNil(t, createWorkflowSpan)

			workflow1Span := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Workflow: wf")
			})
			require.NotNil(t, workflow1Span)

			swfSpan := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Workflow: swf")
			})
			require.NotNil(t, workflow1Span)

			require.Equal(t,
				workflow1Span.SpanContext().SpanID().String(),
				swfSpan.Parent().SpanID().String(),
			)
		},
	},
	{
		name: "Tracing/InnerWorkflowSpansHaveCorrectParent",
		f: func(t *testing.T, ctx context.Context, c *client.Client, w *worker.Worker, b TestBackend) {
			exporter := setupTracing(b)

			wf := func(ctx workflow.Context) error {
				ctx, span := workflow.Tracer(ctx).Start(ctx, "inner-span")
				defer span.End()

				ctx, span2 := workflow.Tracer(ctx).Start(ctx, "inner-span2")
				defer span2.End()

				workflow.Sleep(ctx, time.Microsecond*10)

				return nil
			}
			register(t, ctx, w, []interface{}{wf}, nil)

			instance := runWorkflow(t, ctx, c, wf)
			_, err := client.GetWorkflowResult[any](ctx, c, instance, time.Second*5)
			require.NoError(t, err)

			spans := exporter.GetSpans().Snapshots()
			workflow1Span := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return strings.Contains(span.Name(), "Workflow: 1")
			})
			require.NotNil(t, workflow1Span)

			innerSpan := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return span.Name() == "inner-span"
			})
			require.NotNil(t, innerSpan)
			require.Equal(t, workflow1Span.SpanContext().SpanID(), innerSpan.Parent().SpanID())

			innerSpan2 := findSpan(spans, func(span trace.ReadOnlySpan) bool {
				return span.Name() == "inner-span2"
			})
			require.NotNil(t, innerSpan2)
			require.Equal(t, innerSpan.SpanContext().SpanID(), innerSpan2.Parent().SpanID())
		},
	},
}

func findSpan(spans []trace.ReadOnlySpan, f func(trace.ReadOnlySpan) bool) trace.ReadOnlySpan {
	for _, span := range spans {
		if f(span) {
			return span
		}
	}

	return nil
}
