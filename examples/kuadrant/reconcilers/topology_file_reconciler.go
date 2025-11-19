package reconcilers

import (
	"context"
	"os"
	"sync"

	"go.opentelemetry.io/otel/trace"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"
)

const topologyFile = "topology.dot"

type TopologyFileReconciler struct{}

func (r *TopologyFileReconciler) Reconcile(ctx context.Context, _ []controller.ResourceEvent, topology *machinery.Topology, err error, _ *sync.Map) error {
	logger := controller.TraceLoggerFromContext(ctx).WithName("topology file")

	logger.V(1).Info("writing topology to file", "file", topologyFile)

	file, err := os.Create(topologyFile)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err)
		logger.Error(err, "failed to create topology file", "file", topologyFile)
		return err
	}
	defer file.Close()

	content := topology.ToDot()
	_, err = file.WriteString(content)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err)
		logger.Error(err, "failed to write to topology file", "file", topologyFile)
		return err
	}

	logger.Info("topology file written successfully",
		"file", topologyFile,
		"size.bytes", len(content),
	)

	return nil
}
