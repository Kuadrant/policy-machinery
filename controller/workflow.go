package controller

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/kuadrant/policy-machinery/machinery"
)

// Workflow runs an optional precondition reconciliation function, then dispatches the reconciliation event to
// a list of concurrent reconciliation tasks, and runs an optional postcondition reconciliation function.
//
// If any of the reconciliation functions returns an error, the error is handled by an optional error handler.
// The error passed to the error handler func is conflated with any ocasional error carried over into the call
// to the workflow in the first place. It is up to the error handler to decide how to handle the error and whether
// to supress it or raise it again. Supressed errors cause the workflow to continue running, while raised errors
// interrupt the workflow. If the error handler is nil, the error is raised.
type Workflow struct {
	Precondition  ReconcileFunc
	Tasks         []ReconcileFunc
	Postcondition ReconcileFunc
	ErrorHandler  ReconcileFunc
}

func (w *Workflow) Run(ctx context.Context, resourceEvents []ResourceEvent, topology *machinery.Topology, err error, state *sync.Map) error {
	// run precondition reconcile function
	if w.Precondition != nil {
		if preconditionErr := w.Precondition(ctx, resourceEvents, topology, err, state); preconditionErr != nil {
			if err := w.handle(ctx, resourceEvents, topology, err, state, preconditionErr); err != nil {
				return err
			}
		}
	}

	// dispatch the event to concurrent tasks
	g, groupCtx := errgroup.WithContext(ctx)
	for _, f := range w.Tasks {
		g.Go(func() error {
			return f(groupCtx, resourceEvents, topology, err, state)
		})
	}
	if taskErr := g.Wait(); taskErr != nil {
		if err := w.handle(ctx, resourceEvents, topology, err, state, taskErr); err != nil {
			return err
		}
	}

	// run precondition reconcile function
	if w.Postcondition != nil {
		if postconditionErr := w.Postcondition(ctx, resourceEvents, topology, err, state); postconditionErr != nil {
			if err := w.handle(ctx, resourceEvents, topology, err, state, postconditionErr); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *Workflow) handle(ctx context.Context, resourceEvents []ResourceEvent, topology *machinery.Topology, carryoverErr error, state *sync.Map, workflowErr error) error {
	if w.ErrorHandler != nil {
		return w.ErrorHandler(ctx, resourceEvents, topology, errors.Join(carryoverErr, workflowErr), state)
	}
	return workflowErr
}
