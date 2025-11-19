package controller

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"k8s.io/klog/v2"

	ctrlruntime "sigs.k8s.io/controller-runtime"
)

// CreateAndSetLogger returns a new logger and sets it as the default logger
// for the controller-runtime and klog packages.
func CreateAndSetLogger() logr.Logger {
	logger := logr.Discard()
	zapLogger, err := zap.NewProduction(zap.WithCaller(false))
	if err == nil {
		logger = zapr.NewLogger(zapLogger)
	}
	ctrlruntime.SetLogger(logger)
	klog.SetLogger(logger)
	return logger
}

// LoggerFromContext returns the logger from the context, or a discard logger if
// no logger is found.
func LoggerFromContext(ctx context.Context) logr.Logger {
	logger, ok := ctx.Value(logr.Logger{}).(logr.Logger)
	if !ok {
		return logr.Discard()
	}
	return logger
}

// LoggerIntoContext returns a new context with the logger set.
func LoggerIntoContext(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, logr.Logger{}, logger)
}

// LoggerWithTraceContext enriches a logger with trace and span IDs from the current context.
// If the context contains a valid span, it adds "trace_id" and "span_id" fields to the logger.
// This enables correlation between logs and distributed traces in observability systems.
//
// This is a utility function that users can choose to use for trace-log correlation.
// It does not modify the logger in the context - callers should decide whether to use
// the enriched logger and optionally store it back in context with LoggerIntoContext.
//
// Example usage:
//
//	logger := controller.LoggerFromContext(ctx)
//	logger = controller.LoggerWithTraceContext(ctx, logger)
//	logger.Info("processing request") // Log will include trace_id and span_id
//
// Alternative approaches:
//   - Use otellogr for automatic correlation via OpenTelemetry Logs SDK
//   - Implement custom correlation strategy based on your observability stack
func LoggerWithTraceContext(ctx context.Context, logger logr.Logger) logr.Logger {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		spanCtx := span.SpanContext()
		return logger.WithValues(
			"trace_id", spanCtx.TraceID().String(),
			"span_id", spanCtx.SpanID().String(),
		)
	}
	return logger
}

// TraceLoggerFromContext is a convenience function that returns a logger from context
// with trace context automatically added. This combines LoggerFromContext and
// LoggerWithTraceContext for easier use.
//
// This is an opt-in utility - use this when you want trace-log correlation.
// For standard logging without trace IDs, use LoggerFromContext instead.
//
// Example usage in a reconciler:
//
//	func (r *MyReconciler) Reconcile(ctx context.Context, ...) error {
//	    logger := controller.TraceLoggerFromContext(ctx).WithName("my-component")
//	    logger.Info("processing event") // Log will include trace_id and span_id
//	    // ... reconciliation logic
//	}
func TraceLoggerFromContext(ctx context.Context) logr.Logger {
	logger := LoggerFromContext(ctx)
	return LoggerWithTraceContext(ctx, logger)
}
