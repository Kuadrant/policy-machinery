package machinery

import (
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Targetable is an interface that represents an object that can be targeted by policies.
type Targetable interface {
	Object

	SetPolicies([]Policy)
	Policies() []Policy
}

type GatewayClass struct {
	*gwapiv1.GatewayClass

	attachedPolicies []Policy
}

var _ Targetable = GatewayClass{}

func (g GatewayClass) GetURL() string {
	return UrlFromObject(g)
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

func (g Gateway) GetURL() string {
	return UrlFromObject(g)
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

func (l Listener) GetURL() string {
	return UrlFromObject(l)
}

func (l Listener) GetNamespace() string {
	return l.gateway.GetNamespace()
}

func (l Listener) GetName() string {
	return namespacedName(l.gateway.Name, string(l.Name))
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

func (r HTTPRoute) GetURL() string {
	return UrlFromObject(r)
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
	name             gwapiv1.SectionName // TODO(guicassolato): Use the `name` field of the HTTPRouteRule once it's implemented - https://github.com/kubernetes-sigs/gateway-api/pull/2985
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

func (r HTTPRouteRule) GetURL() string {
	return UrlFromObject(r)
}

func (r HTTPRouteRule) GetNamespace() string {
	return r.httpRoute.GetNamespace()
}

func (r HTTPRouteRule) GetName() string {
	return namespacedName(r.httpRoute.Name, string(r.name))
}

func (r HTTPRouteRule) SetPolicies(policies []Policy) {
	r.attachedPolicies = policies
}

func (r HTTPRouteRule) Policies() []Policy {
	return r.attachedPolicies
}

type Service struct {
	*core.Service

	attachedPolicies []Policy
}

var _ Targetable = Service{}

func (s Service) GetURL() string {
	return UrlFromObject(s)
}

func (s Service) SetPolicies(policies []Policy) {
	s.attachedPolicies = policies
}

func (s Service) Policies() []Policy {
	return s.attachedPolicies
}

type ServicePort struct {
	*core.ServicePort

	service          *Service
	attachedPolicies []Policy
}

var _ Targetable = ServicePort{}

func (p ServicePort) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Kind: "ServicePort",
	}
}

func (p ServicePort) SetGroupVersionKind(schema.GroupVersionKind) {}

func (p ServicePort) GetURL() string {
	return UrlFromObject(p)
}

func (p ServicePort) GetNamespace() string {
	return p.service.GetNamespace()
}

func (p ServicePort) GetName() string {
	return namespacedName(p.service.Name, string(p.Name))
}

func (p ServicePort) SetPolicies(policies []Policy) {
	p.attachedPolicies = policies
}

func (p ServicePort) Policies() []Policy {
	return p.attachedPolicies
}
