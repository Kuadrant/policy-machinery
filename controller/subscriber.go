package controller

import (
	"context"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kuadrant/policy-machinery/machinery"
)

type ResourceEventMatcher struct {
	Kind            *schema.GroupKind
	EventType       *EventType
	ObjectNamespace string
	ObjectName      string
}

// Subscription runs a reconciliation function when the list of events has at least one event in common with
// the list of event matchers. The list of events then propagated to the reconciliation function is filtered
// to the ones the match only.
type Subscription struct {
	ReconcileFunc ReconcileFunc
	Events        []ResourceEventMatcher
}

func (s Subscription) Reconcile(ctx context.Context, resourceEvents []ResourceEvent, topology *machinery.Topology) {
	matchingEvents := lo.Filter(resourceEvents, func(resourceEvent ResourceEvent, _ int) bool {
		return lo.ContainsBy(s.Events, func(m ResourceEventMatcher) bool {
			obj := resourceEvent.OldObject
			if obj == nil {
				obj = resourceEvent.NewObject
			}
			return (m.Kind == nil || *m.Kind == resourceEvent.Kind) &&
				(m.EventType == nil || *m.EventType == resourceEvent.EventType) &&
				(m.ObjectNamespace == "" || m.ObjectNamespace == obj.GetNamespace()) &&
				(m.ObjectName == "" || m.ObjectName == obj.GetName())
		})
	})
	if len(matchingEvents) > 0 && s.ReconcileFunc != nil {
		s.ReconcileFunc(ctx, matchingEvents, topology)
	}
}
