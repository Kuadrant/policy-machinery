package controller

import (
	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/policy-machinery/machinery"
)

func newGatewayAPITopologyBuilder(policyKinds, objectKinds []schema.GroupKind, objectLinks []RuntimeLinkFunc) *gatewayAPITopologyBuilder {
	return &gatewayAPITopologyBuilder{
		policyKinds: policyKinds,
		objectKinds: objectKinds,
		objectLinks: objectLinks,
	}
}

type gatewayAPITopologyBuilder struct {
	policyKinds []schema.GroupKind
	objectKinds []schema.GroupKind
	objectLinks []RuntimeLinkFunc
}

func (t *gatewayAPITopologyBuilder) Build(objs Store) *machinery.Topology {
	gatewayClasses := lo.Map(objs.FilterByGroupKind(GatewayClassKind), RuntimeObjectAs[*gwapiv1.GatewayClass])
	gateways := lo.Map(objs.FilterByGroupKind(GatewayKind), RuntimeObjectAs[*gwapiv1.Gateway])
	httpRoutes := lo.Map(objs.FilterByGroupKind(HTTPRouteKind), RuntimeObjectAs[*gwapiv1.HTTPRoute])
	services := lo.Map(objs.FilterByGroupKind(ServiceKind), RuntimeObjectAs[*core.Service])

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
		policies := lo.Map(objs.FilterByGroupKind(policyKind), RuntimeObjectAs[machinery.Policy])
		opts = append(opts, machinery.WithGatewayAPITopologyPolicies(policies...))
	}

	for i := range t.objectKinds {
		objectKind := t.objectKinds[i]
		objects := lo.FilterMap(objs.FilterByGroupKind(objectKind), func(obj RuntimeObject, _ int) (machinery.Object, bool) {
			object, ok := obj.(machinery.Object)
			if ok {
				return object, ok
			}
			return &Object{obj}, true
		})
		opts = append(opts, machinery.WithGatewayAPITopologyObjects(objects...))
	}

	return machinery.NewGatewayAPITopology(opts...)
}

type Object struct {
	RuntimeObject
}

func (o *Object) GroupVersionKind() schema.GroupVersionKind {
	return o.RuntimeObject.GetObjectKind().GroupVersionKind()
}

func (o *Object) SetGroupVersionKind(schema.GroupVersionKind) {}

func (o *Object) GetNamespace() string {
	return o.RuntimeObject.GetNamespace()
}

func (o *Object) GetName() string {
	return o.RuntimeObject.GetName()
}

func (o *Object) GetURL() string {
	return machinery.UrlFromObject(o)
}
