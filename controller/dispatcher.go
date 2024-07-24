package controller

import (
	"context"
	"sync"

	"github.com/kuadrant/policy-machinery/machinery"
)

// Dispatcher runs an optional precondition reconciliation function and then dispatches the reconciliation event to
// a list of subsequent reconcilers.
type Dispatcher struct {
	PreconditionReconcileFunc CallbackFunc
	ReconcileFuncs            []CallbackFunc
}

func (d *Dispatcher) Dispatch(ctx context.Context, resourceEvent ResourceEvent, topology *machinery.Topology) {
	// run precondition reconcile function
	if d.PreconditionReconcileFunc != nil {
		d.PreconditionReconcileFunc(ctx, resourceEvent, topology)
	}

	// dispatch the event to subsequent reconcilers
	funcs := d.ReconcileFuncs
	waitGroup := &sync.WaitGroup{}
	defer waitGroup.Wait()
	waitGroup.Add(len(funcs))
	for _, f := range funcs {
		go func() {
			defer waitGroup.Done()
			f(ctx, resourceEvent, topology)
		}()
	}
}
