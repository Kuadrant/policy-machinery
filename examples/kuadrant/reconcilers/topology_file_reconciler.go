package reconcilers

import (
	"context"
	"os"
	"sync"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"
)

const topologyFile = "topology.dot"

type TopologyFileReconciler struct{}

func (r *TopologyFileReconciler) Reconcile(ctx context.Context, _ []controller.ResourceEvent, topology *machinery.Topology, err error, _ *sync.Map) error {
	logger := controller.LoggerFromContext(ctx).WithName("topology file")

	file, err := os.Create(topologyFile)
	if err != nil {
		logger.Error(err, "failed to create topology file")
		return err
	}
	defer file.Close()

	_, err = file.WriteString(topology.ToDot())
	if err != nil {
		logger.Error(err, "failed to write to topology file")
		return err
	}

	return nil
}
