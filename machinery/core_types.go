package machinery

import (
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var (
	ServiceGroupKind     = schema.GroupKind{Kind: "Service"}
	ServicePortGroupKind = schema.GroupKind{Kind: "ServicePort"}
)

// These are wrappers for Core API types so instances can be used as targetables in the topology.

type Namespace struct {
	*core.Namespace

	attachedPolicies []Policy
}

var _ Targetable = &Namespace{}

func (n *Namespace) GetURL() string {
	return UrlFromObject(n)
}

func (n *Namespace) SetPolicies(policies []Policy) {
	n.attachedPolicies = policies
}

func (n *Namespace) Policies() []Policy {
	return n.attachedPolicies
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
