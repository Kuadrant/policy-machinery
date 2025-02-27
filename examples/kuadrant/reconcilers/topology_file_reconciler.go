package reconcilers

import (
	"context"
	"os"
	"sync"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"
)

const topologyFile = "topology.dot"
const topologyFileJSON = "topology.json"

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

	jsonFile, err := os.Create(topologyFileJSON)
	if err != nil {
		logger.Error(err, "failed to create topology json file")
		return err
	}
	defer jsonFile.Close()

	tJson, err := topology.ToJSON()
	if err != nil {
		logger.Error(err, "failed to create topology json file")
		return err
	}

	_, err = jsonFile.WriteString(tJson)
	if err != nil {
		logger.Error(err, "failed to write to topology json file")
		return err
	}

	return nil
}
