package controller

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
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
