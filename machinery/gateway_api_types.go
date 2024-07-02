package machinery

import (
	"fmt"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const nameSectionNameURLSeparator = '#'

// These are wrappers for Gateway API types so instances can be used as targetables in the topology.
// Targateables typically store back references to the policies that are attached to them.
// The implementation of GetURL() must return a unique identifier for the wrapped object that matches the one
// generated by policy targetRefs that implement the PolicyTargetReference interface for values pointing to the object.

type GatewayClass struct {
	*gwapiv1.GatewayClass

	attachedPolicies []Policy
}

var _ Targetable = &GatewayClass{}

func (g *GatewayClass) GetURL() string {
	return UrlFromObject(g)
}

func (g *GatewayClass) SetPolicies(policies []Policy) {
	g.attachedPolicies = policies
}

func (g *GatewayClass) Policies() []Policy {
	return g.attachedPolicies
}

type Gateway struct {
	*gwapiv1.Gateway

	attachedPolicies []Policy
}

var _ Targetable = &Gateway{}

func (g *Gateway) GetURL() string {
	return UrlFromObject(g)
}

func (g *Gateway) SetPolicies(policies []Policy) {
	g.attachedPolicies = policies
}

func (g *Gateway) Policies() []Policy {
	return g.attachedPolicies
}

type Listener struct {
	*gwapiv1.Listener

	Gateway          *Gateway
	attachedPolicies []Policy
}

var _ Targetable = &Listener{}

func (l *Listener) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   gwapiv1.GroupName,
		Version: gwapiv1.GroupVersion.Version,
		Kind:    "Listener",
	}
}

func (l *Listener) SetGroupVersionKind(schema.GroupVersionKind) {}

func (l *Listener) GetURL() string {
	return namespacedSectionName(UrlFromObject(l.Gateway), l.Name)
}

func (l *Listener) GetNamespace() string {
	return l.Gateway.GetNamespace()
}

func (l *Listener) GetName() string {
	return namespacedSectionName(l.Gateway.GetName(), l.Name)
}

func (l *Listener) SetPolicies(policies []Policy) {
	l.attachedPolicies = policies
}

func (l *Listener) Policies() []Policy {
	return l.attachedPolicies
}

type HTTPRoute struct {
	*gwapiv1.HTTPRoute

	attachedPolicies []Policy
}

var _ Targetable = &HTTPRoute{}

func (r *HTTPRoute) GetURL() string {
	return UrlFromObject(r)
}

func (r *HTTPRoute) SetPolicies(policies []Policy) {
	r.attachedPolicies = policies
}

func (r *HTTPRoute) Policies() []Policy {
	return r.attachedPolicies
}

type HTTPRouteRule struct {
	*gwapiv1.HTTPRouteRule

	HTTPRoute        *HTTPRoute
	Name             gwapiv1.SectionName // TODO(guicassolato): Use the `name` field of the HTTPRouteRule once it's implemented - https://github.com/kubernetes-sigs/gateway-api/pull/2985
	attachedPolicies []Policy
}

var _ Targetable = &HTTPRouteRule{}

func (r *HTTPRouteRule) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   gwapiv1.GroupName,
		Version: gwapiv1.GroupVersion.Version,
		Kind:    "HTTPRouteRule",
	}
}

func (r *HTTPRouteRule) SetGroupVersionKind(schema.GroupVersionKind) {}

func (r *HTTPRouteRule) GetURL() string {
	return namespacedSectionName(UrlFromObject(r.HTTPRoute), r.Name)
}

func (r *HTTPRouteRule) GetNamespace() string {
	return r.HTTPRoute.GetNamespace()
}

func (r *HTTPRouteRule) GetName() string {
	return namespacedSectionName(r.HTTPRoute.Name, r.Name)
}

func (r *HTTPRouteRule) SetPolicies(policies []Policy) {
	r.attachedPolicies = policies
}

func (r *HTTPRouteRule) Policies() []Policy {
	return r.attachedPolicies
}

type Service struct {
	*core.Service

	attachedPolicies []Policy
}

var _ Targetable = &Service{}

func (s *Service) GetURL() string {
	return UrlFromObject(s)
}

func (s *Service) SetPolicies(policies []Policy) {
	s.attachedPolicies = policies
}

func (s *Service) Policies() []Policy {
	return s.attachedPolicies
}

type ServicePort struct {
	*core.ServicePort

	Service          *Service
	attachedPolicies []Policy
}

var _ Targetable = &ServicePort{}

func (p *ServicePort) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Kind: "ServicePort",
	}
}

func (p *ServicePort) SetGroupVersionKind(schema.GroupVersionKind) {}

func (p *ServicePort) GetURL() string {
	return namespacedSectionName(UrlFromObject(p.Service), gwapiv1.SectionName(p.Name))
}

func (p *ServicePort) GetNamespace() string {
	return p.Service.GetNamespace()
}

func (p *ServicePort) GetName() string {
	return namespacedSectionName(p.Service.Name, gwapiv1.SectionName(p.Name))
}

func (p *ServicePort) SetPolicies(policies []Policy) {
	p.attachedPolicies = policies
}

func (p *ServicePort) Policies() []Policy {
	return p.attachedPolicies
}

// These are Gateway API target reference types that implement the PolicyTargetReference interface, so policies'
// targetRef instances can be treated as Objects whose GetURL() functions return the unique identifier of the
// corresponding targetable the reference points to.
// This is the reason why GetURL() was adopted to get the unique identifiers of topology objects instead of more
// obvious Kubernetes objects' GetUID() (k8s.io/apimachinery/pkg/apis/meta/v1).

type NamespacedPolicyTargetReference struct {
	gwapiv1alpha2.NamespacedPolicyTargetReference
	PolicyNamespace string
}

var _ PolicyTargetReference = NamespacedPolicyTargetReference{}

func (t NamespacedPolicyTargetReference) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group: string(t.Group),
		Kind:  string(t.Kind),
	}
}

func (t NamespacedPolicyTargetReference) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	t.Group = gwapiv1alpha2.Group(gvk.Group)
	t.Kind = gwapiv1alpha2.Kind(gvk.Kind)
}

func (t NamespacedPolicyTargetReference) GetURL() string {
	return UrlFromObject(t)
}

func (t NamespacedPolicyTargetReference) GetNamespace() string {
	return string(ptr.Deref(t.Namespace, gwapiv1alpha2.Namespace(t.PolicyNamespace)))
}

func (t NamespacedPolicyTargetReference) GetName() string {
	return string(t.NamespacedPolicyTargetReference.Name)
}

type LocalPolicyTargetReference struct {
	gwapiv1alpha2.LocalPolicyTargetReference
	PolicyNamespace string
}

var _ PolicyTargetReference = LocalPolicyTargetReference{}

func (t LocalPolicyTargetReference) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group: string(t.Group),
		Kind:  string(t.Kind),
	}
}

func (t LocalPolicyTargetReference) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	t.Group = gwapiv1alpha2.Group(gvk.Group)
	t.Kind = gwapiv1alpha2.Kind(gvk.Kind)
}

func (t LocalPolicyTargetReference) GetURL() string {
	return UrlFromObject(t)
}

func (t LocalPolicyTargetReference) GetNamespace() string {
	return t.PolicyNamespace
}

func (t LocalPolicyTargetReference) GetName() string {
	return string(t.LocalPolicyTargetReference.Name)
}

type LocalPolicyTargetReferenceWithSectionName struct {
	gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName
	PolicyNamespace string
}

var _ PolicyTargetReference = LocalPolicyTargetReferenceWithSectionName{}

func (t LocalPolicyTargetReferenceWithSectionName) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group: string(t.Group),
		Kind:  string(t.Kind),
	}
}

func (t LocalPolicyTargetReferenceWithSectionName) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	t.Group = gwapiv1alpha2.Group(gvk.Group)
	t.Kind = gwapiv1alpha2.Kind(gvk.Kind)
}

func (t LocalPolicyTargetReferenceWithSectionName) GetURL() string {
	return UrlFromObject(t)
}

func (t LocalPolicyTargetReferenceWithSectionName) GetNamespace() string {
	return t.PolicyNamespace
}

func (t LocalPolicyTargetReferenceWithSectionName) GetName() string {
	if t.SectionName == nil {
		return string(t.LocalPolicyTargetReference.Name)
	}
	return namespacedSectionName(string(t.LocalPolicyTargetReference.Name), *t.SectionName)
}

func namespacedSectionName(namespace string, sectionName gwapiv1.SectionName) string {
	return fmt.Sprintf("%s%s%s", namespace, string(nameSectionNameURLSeparator), sectionName)
}