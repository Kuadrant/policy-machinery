package controller

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/kuadrant/policy-machinery/machinery"
)

type tracerKey struct{}

// TracerFromContext returns the tracer from the context, or a no-op tracer if
// no tracer is found.
func TracerFromContext(ctx context.Context) trace.Tracer {
	tracer, ok := ctx.Value(tracerKey{}).(trace.Tracer)
	if !ok {
		return noop.NewTracerProvider().Tracer("")
	}
	return tracer
}

// TracerIntoContext returns a new context with the tracer set.
func TracerIntoContext(ctx context.Context, tracer trace.Tracer) context.Context {
	return context.WithValue(ctx, tracerKey{}, tracer)
}

// TraceReconcileFunc wraps a ReconcileFunc with tracing instrumentation.
// It extracts the tracer from the context, starts a span with the given name,
// and automatically records errors and carryover error context.
//
// Example usage:
//
//	reconciler := controller.TraceReconcileFunc("my-reconciler", func(ctx context.Context, events []ResourceEvent, topology *machinery.Topology, err error, state *sync.Map) error {
//	    // Your reconciliation logic here
//	    return nil
//	})
func TraceReconcileFunc(spanName string, fn ReconcileFunc) ReconcileFunc {
	return func(ctx context.Context, events []ResourceEvent, topology *machinery.Topology, errIn error, state *sync.Map) error {
		tracer := TracerFromContext(ctx)

		ctx, span := tracer.Start(ctx, spanName)
		defer span.End()

		// Record carryover error
		if errIn != nil {
			span.RecordError(errIn)
			span.SetAttributes(attribute.Bool("error.carryover", true))
		}

		// Execute the wrapped function
		err := fn(ctx, events, topology, errIn, state)

		// Record result
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}
