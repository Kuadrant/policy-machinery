package controller

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/kuadrant/policy-machinery/machinery"
)

// Workflow runs an optional precondition reconciliation function, then dispatches the reconciliation event to
// a list of concurrent reconciliation tasks, and runs an optional postcondition reconciliation function.
type Workflow struct {
	Precondition  ReconcileFunc
	Tasks         []ReconcileFunc
	Postcondition ReconcileFunc
}

func (d *Workflow) Run(ctx context.Context, resourceEvents []ResourceEvent, topology *machinery.Topology, err error, state *sync.Map) error {
	// run precondition reconcile function
	if d.Precondition != nil {
		if err := d.Precondition(ctx, resourceEvents, topology, err, state); err != nil {
			return err
		}
	}

	// dispatch the event to concurrent tasks
	g, groupCtx := errgroup.WithContext(ctx)
	for _, f := range d.Tasks {
		g.Go(func() error {
			return f(groupCtx, resourceEvents, topology, err, state)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// run precondition reconcile function
	if d.Postcondition != nil {
		if err := d.Postcondition(ctx, resourceEvents, topology, err, state); err != nil {
			return err
		}
	}

	return nil
}
