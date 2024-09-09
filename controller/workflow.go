package controller

import (
	"context"
	"sync"

	"github.com/kuadrant/policy-machinery/machinery"
)

// Workflow runs an optional precondition reconciliation function, then dispatches the reconciliation event to
// a list of concurrent reconciliation tasks, and runs an optional postcondition reconciliation function.
type Workflow struct {
	Precondition  ReconcileFunc
	Tasks         []ReconcileFunc
	Postcondition ReconcileFunc
}

func (d *Workflow) Run(ctx context.Context, resourceEvents []ResourceEvent, topology *machinery.Topology, state *sync.Map, err error) {
	// run precondition reconcile function
	if d.Precondition != nil {
		d.Precondition(ctx, resourceEvents, topology, state, err)
	}

	// dispatch the event to concurrent tasks
	funcs := d.Tasks
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(len(funcs))
	for _, f := range funcs {
		go func() {
			defer waitGroup.Done()
			f(ctx, resourceEvents, topology, state, err)
		}()
	}
	waitGroup.Wait()

	// run precondition reconcile function
	if d.Postcondition != nil {
		d.Postcondition(ctx, resourceEvents, topology, state, err)
	}
}
