package reconcilers

import (
	"context"
	"log"
	"os"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"
)

const topologyFile = "topology.dot"

type TopologyFileReconciler struct{}

func (r *TopologyFileReconciler) Reconcile(_ context.Context, _ []controller.ResourceEvent, topology *machinery.Topology) {
	file, err := os.Create(topologyFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	_, err = file.WriteString(topology.ToDot())
	if err != nil {
		log.Fatal(err)
	}
}
