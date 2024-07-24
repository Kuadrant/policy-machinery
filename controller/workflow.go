package controller

import (
	"context"
	"sync"

	"github.com/kuadrant/policy-machinery/machinery"
)

// Workflow runs an optional precondition reconciliation function, then dispatches the reconciliation event to
// a list of concurrent reconciliation tasks, and runs an optional postcondition reconciliation function.
type Workflow struct {
	Precondition  CallbackFunc
	Tasks         []CallbackFunc
	Postcondition CallbackFunc
}

func (d *Workflow) Run(ctx context.Context, resourceEvent ResourceEvent, topology *machinery.Topology) {
	// run precondition reconcile function
	if d.Precondition != nil {
		d.Precondition(ctx, resourceEvent, topology)
	}

	// dispatch the event to concurrent tasks
	funcs := d.Tasks
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(len(funcs))
	for _, f := range funcs {
		go func() {
			defer waitGroup.Done()
			f(ctx, resourceEvent, topology)
		}()
	}
	waitGroup.Wait()

	// run precondition reconcile function
	if d.Postcondition != nil {
		d.Postcondition(ctx, resourceEvent, topology)
	}
}
