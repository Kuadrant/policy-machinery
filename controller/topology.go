package controller

import (
	"sync"

	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/policy-machinery/machinery"
)

func NewGatewayAPITopology(policyKinds, objectKinds []schema.GroupKind, objectLinks []RuntimeLinkFunc) *GatewayAPITopology {
	return &GatewayAPITopology{
		topology:    machinery.NewTopology(),
		policyKinds: policyKinds,
		objectKinds: objectKinds,
		objectLinks: objectLinks,
	}
}

type GatewayAPITopology struct {
	mu          sync.RWMutex
	topology    *machinery.Topology
	policyKinds []schema.GroupKind
	objectKinds []schema.GroupKind
	objectLinks []RuntimeLinkFunc
}

func (t *GatewayAPITopology) Refresh(objs Store) {
	t.mu.Lock()
	defer t.mu.Unlock()

	gatewayClasses := lo.FilterMap(lo.Values(objs[schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "GatewayClass"}]), func(obj RuntimeObject, _ int) (*gwapiv1.GatewayClass, bool) {
		gc, ok := obj.(*gwapiv1.GatewayClass)
		if !ok {
			return nil, false
		}
		return gc, true
	})

	gateways := lo.FilterMap(lo.Values(objs[schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"}]), func(obj RuntimeObject, _ int) (*gwapiv1.Gateway, bool) {
		gw, ok := obj.(*gwapiv1.Gateway)
		if !ok {
			return nil, false
		}
		return gw, true
	})

	httpRoutes := lo.FilterMap(lo.Values(objs[schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"}]), func(obj RuntimeObject, _ int) (*gwapiv1.HTTPRoute, bool) {
		httpRoute, ok := obj.(*gwapiv1.HTTPRoute)
		if !ok {
			return nil, false
		}
		return httpRoute, true
	})

	services := lo.FilterMap(lo.Values(objs[schema.GroupKind{Group: core.GroupName, Kind: "Service"}]), func(obj RuntimeObject, _ int) (*core.Service, bool) {
		service, ok := obj.(*core.Service)
		if !ok {
			return nil, false
		}
		return service, true
	})

	linkFuncs := lo.Map(t.objectLinks, func(linkFunc RuntimeLinkFunc, _ int) machinery.LinkFunc {
		return linkFunc(objs)
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
		policies := lo.FilterMap(lo.Values(objs[policyKind]), func(obj RuntimeObject, _ int) (machinery.Policy, bool) {
			policy, ok := obj.(machinery.Policy)
			return policy, ok
		})

		opts = append(opts, machinery.WithGatewayAPITopologyPolicies(policies...))
	}

	for i := range t.objectKinds {
		objectKind := t.objectKinds[i]
		objects := lo.FilterMap(lo.Values(objs[objectKind]), func(obj RuntimeObject, _ int) (machinery.Object, bool) {
			object, ok := obj.(machinery.Object)
			if ok {
				return object, ok
			}
			return &genericObject{obj}, true
		})

		opts = append(opts, machinery.WithGatewayAPITopologyObjects(objects...))
	}

	t.topology = machinery.NewGatewayAPITopology(opts...)
}

func (t *GatewayAPITopology) Get() *machinery.Topology {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.topology == nil {
		return nil
	}
	topology := *t.topology
	return &topology
}

type genericObject struct {
	obj RuntimeObject
}

func (g *genericObject) GroupVersionKind() schema.GroupVersionKind {
	return g.obj.GetObjectKind().GroupVersionKind()
}

func (g *genericObject) SetGroupVersionKind(schema.GroupVersionKind) {}

func (g *genericObject) GetNamespace() string {
	return g.obj.GetNamespace()
}

func (g *genericObject) GetName() string {
	return g.obj.GetName()
}

func (g *genericObject) GetURL() string {
	return machinery.UrlFromObject(g)
}
