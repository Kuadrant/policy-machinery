package machinery

import (
	"fmt"

	"github.com/kuadrant/kuadrant-operator/pkg/library/dag"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Targetable is an interface that represents an object that can be targeted by policies.
type Targetable interface {
	schema.ObjectKind
	dag.Node

	SetPolicies([]Policy)
	Policies() []Policy
}

type GatewayClass struct {
	*gwapiv1.GatewayClass

	attachedPolicies []Policy
}

var _ Targetable = GatewayClass{}

func (g GatewayClass) ID() string {
	return nodeID(g, g.Name)
}

func (g GatewayClass) SetPolicies(policies []Policy) {
	g.attachedPolicies = policies
}

func (g GatewayClass) Policies() []Policy {
	return g.attachedPolicies
}

type Gateway struct {
	*gwapiv1.Gateway

	attachedPolicies []Policy
}

var _ Targetable = Gateway{}

func (g Gateway) ID() string {
	return nodeID(g, namespacedName(g.Namespace, g.Name))
}

func (g Gateway) SetPolicies(policies []Policy) {
	g.attachedPolicies = policies
}

func (g Gateway) Policies() []Policy {
	return g.attachedPolicies
}

type Listener struct {
	*gwapiv1.Listener

	gateway          *Gateway
	attachedPolicies []Policy
}

var _ Targetable = Listener{}

func (l Listener) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   gwapiv1.GroupName,
		Version: gwapiv1.GroupVersion.Version,
		Kind:    "Listener",
	}
}

func (l Listener) SetGroupVersionKind(schema.GroupVersionKind) {}

func (l Listener) ID() string {
	return nodeID(l, namespacedNameWithSectionName(l.gateway.Namespace, l.gateway.Name, string(l.Name)))
}

func (l Listener) SetPolicies(policies []Policy) {
	l.attachedPolicies = policies
}

func (l Listener) Policies() []Policy {
	return l.attachedPolicies
}

type HTTPRoute struct {
	*gwapiv1.HTTPRoute

	attachedPolicies []Policy
}

var _ Targetable = HTTPRoute{}

func (r HTTPRoute) ID() string {
	return nodeID(r, namespacedName(r.Namespace, r.Name))
}

func (r HTTPRoute) SetPolicies(policies []Policy) {
	r.attachedPolicies = policies
}

func (r HTTPRoute) Policies() []Policy {
	return r.attachedPolicies
}

type HTTPRouteRule struct {
	*gwapiv1.HTTPRouteRule

	httpRoute        *HTTPRoute
	name             string // TODO(guicassolato): Use the `name` field of the HTTPRouteRule once it's implemented - https://github.com/kubernetes-sigs/gateway-api/pull/2985
	attachedPolicies []Policy
}

var _ Targetable = HTTPRouteRule{}

func (r HTTPRouteRule) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   gwapiv1.GroupName,
		Version: gwapiv1.GroupVersion.Version,
		Kind:    "HTTPRouteRule",
	}
}

func (r HTTPRouteRule) SetGroupVersionKind(schema.GroupVersionKind) {}

func (r HTTPRouteRule) ID() string {
	return nodeID(r, namespacedNameWithSectionName(r.httpRoute.Namespace, r.httpRoute.Name, string(r.name)))
}

func (r HTTPRouteRule) SetPolicies(policies []Policy) {
	r.attachedPolicies = policies
}

func (r HTTPRouteRule) Policies() []Policy {
	return r.attachedPolicies
}

type Backend struct {
	*core.Service // TODO(guicassolato): Other types of backends

	attachedPolicies []Policy
}

var _ Targetable = Backend{}

func (b Backend) ID() string {
	return nodeID(b, namespacedName(b.Namespace, b.Name))
}

func (b Backend) SetPolicies(policies []Policy) {
	b.attachedPolicies = policies
}

func (b Backend) Policies() []Policy {
	return b.attachedPolicies
}

type BackendPort struct {
	*core.ServicePort

	backend          *Backend
	attachedPolicies []Policy
}

var _ Targetable = BackendPort{}

func (p BackendPort) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Kind: "BackendPort",
	}
}

func (p BackendPort) SetGroupVersionKind(schema.GroupVersionKind) {}

func (p BackendPort) ID() string {
	return nodeID(p, namespacedNameWithSectionName(p.backend.Namespace, p.backend.Name, string(p.Name)))
}

func (p BackendPort) SetPolicies(policies []Policy) {
	p.attachedPolicies = policies
}

func (p BackendPort) Policies() []Policy {
	return p.attachedPolicies
}

func nodeID(obj schema.ObjectKind, objectUniqueName string) string {
	return fmt.Sprintf("%s#%s", objectKind(obj), objectUniqueName)
}
