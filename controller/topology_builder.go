package controller

import (
	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/policy-machinery/machinery"
)

func newGatewayAPITopologyBuilder(policyKinds, objectKinds []schema.GroupKind, objectLinks []LinkFunc) *gatewayAPITopologyBuilder {
	return &gatewayAPITopologyBuilder{
		policyKinds: policyKinds,
		objectKinds: objectKinds,
		objectLinks: objectLinks,
	}
}

type gatewayAPITopologyBuilder struct {
	policyKinds []schema.GroupKind
	objectKinds []schema.GroupKind
	objectLinks []LinkFunc
}

func (t *gatewayAPITopologyBuilder) Build(objs Store) (*machinery.Topology, error) {
	gatewayClasses := lo.Map(objs.FilterByGroupKind(machinery.GatewayClassGroupKind), ObjectAs[*gwapiv1.GatewayClass])
	gateways := lo.Map(objs.FilterByGroupKind(machinery.GatewayGroupKind), ObjectAs[*gwapiv1.Gateway])
	httpRoutes := lo.Map(objs.FilterByGroupKind(machinery.HTTPRouteGroupKind), ObjectAs[*gwapiv1.HTTPRoute])
	services := lo.Map(objs.FilterByGroupKind(machinery.ServiceGroupKind), ObjectAs[*core.Service])

	linkFuncs := lo.Map(t.objectLinks, func(f LinkFunc, _ int) machinery.LinkFunc {
		return f(objs)
	})

	opts := []machinery.GatewayAPITopologyOptionsFunc{
		machinery.WithGatewayClasses(gatewayClasses...),
		machinery.WithGateways(gateways...),
		machinery.WithHTTPRoutes(httpRoutes...),
		machinery.WithServices(services...),
		machinery.ExpandGatewayListeners(),
		machinery.ExpandHTTPRouteRules(),
		machinery.ExpandServicePorts(),
		machinery.WithGatewayAPITopologyLinks(linkFuncs...),
	}

	for i := range t.policyKinds {
		policyKind := t.policyKinds[i]
		policies := lo.Map(objs.FilterByGroupKind(policyKind), ObjectAs[machinery.Policy])
		opts = append(opts, machinery.WithGatewayAPITopologyPolicies(policies...))
	}

	for i := range t.objectKinds {
		objectKind := t.objectKinds[i]
		objects := lo.FilterMap(objs.FilterByGroupKind(objectKind), func(obj Object, _ int) (machinery.Object, bool) {
			object, ok := obj.(machinery.Object)
			if ok {
				return object, ok
			}
			return &RuntimeObject{obj}, true
		})
		opts = append(opts, machinery.WithGatewayAPITopologyObjects(objects...))
	}

	return machinery.NewGatewayAPITopology(opts...)
}
