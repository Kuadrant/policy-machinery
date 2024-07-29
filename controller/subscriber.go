package controller

import (
	"context"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kuadrant/policy-machinery/machinery"
)

type ResourceEventMatcher struct {
	Resource        *schema.GroupVersionResource
	EventType       *EventType
	ObjectNamespace string
	ObjectName      string
}

type Subscription struct {
	ReconcileFunc CallbackFunc
	Events        []ResourceEventMatcher
}

// Subscriber calls the reconciler function of the first subscription that matches the event
type Subscriber []Subscription

func (s Subscriber) Reconcile(ctx context.Context, resourceEvent ResourceEvent, topology *machinery.Topology) {
	subscription, found := lo.Find(s, func(subscription Subscription) bool {
		_, found := lo.Find(subscription.Events, func(m ResourceEventMatcher) bool {
			obj := resourceEvent.OldObject
			if obj == nil {
				obj = resourceEvent.NewObject
			}
			return (m.Resource == nil || *m.Resource == resourceEvent.Resource) &&
				(m.EventType == nil || *m.EventType == resourceEvent.EventType) &&
				(m.ObjectNamespace == "" || m.ObjectNamespace == obj.GetNamespace()) &&
				(m.ObjectName == "" || m.ObjectName == obj.GetName())
		})
		return found
	})
	if found && subscription.ReconcileFunc != nil {
		subscription.ReconcileFunc(ctx, resourceEvent, topology)
	}
}
