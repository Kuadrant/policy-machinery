package machinery

import (
	"fmt"

	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type GatewayAPITopologyOptions struct {
	GatewayClasses []*GatewayClass
	Gateways       []*Gateway
	HTTPRoutes     []*HTTPRoute
	Services       []*Service
	Policies       []Policy
	Objects        []Object
	Links          []LinkFunc

	ExpandGatewayListeners bool
	ExpandHTTPRouteRules   bool
	ExpandServicePorts     bool
}

type GatewayAPITopologyOptionsFunc func(*GatewayAPITopologyOptions)

// WithGatewayClasses adds gateway classes to the options to initialize a new Gateway API topology.
func WithGatewayClasses(gatewayClasses ...*gwapiv1.GatewayClass) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.GatewayClasses = append(o.GatewayClasses, lo.Map(gatewayClasses, func(gatewayClass *gwapiv1.GatewayClass, _ int) *GatewayClass {
			return &GatewayClass{GatewayClass: gatewayClass}
		})...)
	}
}

// WithGateways adds gateways to the options to initialize a new Gateway API topology.
func WithGateways(gateways ...*gwapiv1.Gateway) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.Gateways = append(o.Gateways, lo.Map(gateways, func(gateway *gwapiv1.Gateway, _ int) *Gateway {
			return &Gateway{Gateway: gateway}
		})...)
	}
}

// WithHTTPRoutes adds HTTP routes to the options to initialize a new Gateway API topology.
func WithHTTPRoutes(httpRoutes ...*gwapiv1.HTTPRoute) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.HTTPRoutes = append(o.HTTPRoutes, lo.Map(httpRoutes, func(httpRoute *gwapiv1.HTTPRoute, _ int) *HTTPRoute {
			return &HTTPRoute{HTTPRoute: httpRoute}
		})...)
	}
}

// WithServices adds services to the options to initialize a new Gateway API topology.
func WithServices(services ...*core.Service) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.Services = append(o.Services, lo.Map(services, func(service *core.Service, _ int) *Service {
			return &Service{Service: service}
		})...)
	}
}

// WithGatewayAPITopologyPolicies adds policies to the options to initialize a new Gateway API topology.
func WithGatewayAPITopologyPolicies(policies ...Policy) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.Policies = append(o.Policies, policies...)
	}
}

// WithGatewayAPITopologyObjects adds objects to the options to initialize a new Gateway API topology.
// Do not use this function to add targetables or policies.
// Use WithGatewayAPITopologyLinks to define the relationships between objects of any kind.
func WithGatewayAPITopologyObjects(objects ...Object) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.Objects = append(o.Objects, objects...)
	}
}

// WithLinks adds link functions to the options to initialize a new Gateway API topology.
func WithGatewayAPITopologyLinks(links ...LinkFunc) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.Links = append(o.Links, links...)
	}
}

// ExpandGatewayListeners adds targetable gateway listeners to the options to initialize a new Gateway API topology.
func ExpandGatewayListeners() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.ExpandGatewayListeners = true
	}
}

// ExpandHTTPRouteRules adds targetable HTTP route rules to the options to initialize a new Gateway API topology.
func ExpandHTTPRouteRules() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.ExpandHTTPRouteRules = true
	}
}

// ExpandServicePorts adds targetable service ports to the options to initialize a new Gateway API topology.
func ExpandServicePorts() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.ExpandServicePorts = true
	}
}

// NewGatewayAPITopology returns a topology of Gateway API objects and attached policies.
//
// The links between the targetables are established based on the relationships defined by Gateway API.
//
// Principal objects like Gateways, HTTPRoutes and Services can be expanded to automatically include their targetable
// sections (listeners, route rules, service ports) as independent objects in the topology, by supplying the
// corresponding options ExpandGatewayListeners(), ExpandHTTPRouteRules(), and ExpandServicePorts().
// The links will then be established accordingly. E.g.:
//   - Without expanding Gateway listeners (default): Gateway -> HTTPRoute links.
//   - Expanding Gateway listeners: Gateway -> Listener and Listener -> HTTPRoute links.
func NewGatewayAPITopology(options ...GatewayAPITopologyOptionsFunc) *Topology {
	o := &GatewayAPITopologyOptions{}
	for _, f := range options {
		f(o)
	}

	opts := []TopologyOptionsFunc{
		WithObjects(o.Objects...),
		WithPolicies(o.Policies...),
		WithTargetables(o.GatewayClasses...),
		WithTargetables(o.Gateways...),
		WithTargetables(o.HTTPRoutes...),
		WithTargetables(o.Services...),
		WithLinks(o.Links...),
		WithLinks(LinkGatewayClassToGatewayFunc(o.GatewayClasses)), // GatewayClass -> Gateway
	}

	if o.ExpandGatewayListeners {
		listeners := lo.FlatMap(o.Gateways, ListenersFromGatewayFunc)
		opts = append(opts, WithTargetables(listeners...))
		opts = append(opts, WithLinks(
			LinkGatewayToListenerFunc(),                        // Gateway -> Listener
			LinkListenerToHTTPRouteFunc(o.Gateways, listeners), // Listener -> HTTPRoute
		))
	} else {
		opts = append(opts, WithLinks(LinkGatewayToHTTPRouteFunc(o.Gateways))) // Gateway -> HTTPRoute
	}

	if o.ExpandHTTPRouteRules {
		httpRouteRules := lo.FlatMap(o.HTTPRoutes, HTTPRouteRulesFromHTTPRouteFunc)
		opts = append(opts, WithTargetables(httpRouteRules...))
		opts = append(opts, WithLinks(LinkHTTPRouteToHTTPRouteRuleFunc())) // HTTPRoute -> HTTPRouteRule

		if o.ExpandServicePorts {
			servicePorts := lo.FlatMap(o.Services, ServicePortsFromBackendFunc)
			opts = append(opts, WithTargetables(servicePorts...))
			opts = append(opts, WithLinks(
				LinkHTTPRouteRuleToServicePortFunc(httpRouteRules),   // HTTPRouteRule -> ServicePort
				LinkHTTPRouteRuleToServiceFunc(httpRouteRules, true), // HTTPRouteRule -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkHTTPRouteRuleToServiceFunc(httpRouteRules, false))) // HTTPRouteRule -> Service
		}
	} else {
		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkHTTPRouteToServicePortFunc(o.HTTPRoutes),   // HTTPRoute -> ServicePort
				LinkHTTPRouteToServiceFunc(o.HTTPRoutes, true), // HTTPRoute -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkHTTPRouteToServiceFunc(o.HTTPRoutes, false))) // HTTPRoute -> Service
		}
	}

	if o.ExpandServicePorts {
		opts = append(opts, WithLinks(LinkServiceToServicePortFunc())) // Service -> ServicePort
	}

	return NewTopology(opts...)
}

// ListenersFromGatewayFunc returns a list of targetable listeners from a targetable gateway.
func ListenersFromGatewayFunc(gateway *Gateway, _ int) []*Listener {
	return lo.Map(gateway.Spec.Listeners, func(listener gwapiv1.Listener, _ int) *Listener {
		return &Listener{
			Listener: &listener,
			Gateway:  gateway,
		}
	})
}

// HTTPRouteRulesFromHTTPRouteFunc returns a list of targetable HTTPRouteRules from a targetable HTTPRoute.
func HTTPRouteRulesFromHTTPRouteFunc(httpRoute *HTTPRoute, _ int) []*HTTPRouteRule {
	return lo.Map(httpRoute.Spec.Rules, func(rule gwapiv1.HTTPRouteRule, i int) *HTTPRouteRule {
		return &HTTPRouteRule{
			HTTPRouteRule: &rule,
			HTTPRoute:     httpRoute,
			Name:          gwapiv1.SectionName(fmt.Sprintf("rule-%d", i+1)),
		}
	})
}

// ServicePortsFromBackendFunc returns a list of targetable service ports from a targetable Service.
func ServicePortsFromBackendFunc(service *Service, _ int) []*ServicePort {
	return lo.Map(service.Spec.Ports, func(port core.ServicePort, _ int) *ServicePort {
		return &ServicePort{
			ServicePort: &port,
			Service:     service,
		}
	})
}

// LinkGatewayClassToGatewayFunc returns a link function that teaches a topology how to link Gateways from known
// GatewayClasses, based on the Gateway's `gatewayClassName` field.
func LinkGatewayClassToGatewayFunc(gatewayClasses []*GatewayClass) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "GatewayClass"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		Func: func(child Object) []Object {
			gateway := child.(*Gateway)
			gatewayClass, ok := lo.Find(gatewayClasses, func(gc *GatewayClass) bool {
				return gc.Name == string(gateway.Spec.GatewayClassName)
			})
			if ok {
				return []Object{gatewayClass}
			}
			return nil
		},
	}
}

// LinkGatewayToHTTPRouteFunc returns a link function that teaches a topology how to link HTTPRoutes from known
// Gateways, based on the HTTPRoute's `parentRefs` field.
func LinkGatewayToHTTPRouteFunc(gateways []*Gateway) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		Func: func(child Object) []Object {
			httpRoute := child.(*HTTPRoute)
			return lo.FilterMap(httpRoute.Spec.ParentRefs, func(parentRef gwapiv1.ParentReference, _ int) (Object, bool) {
				parentRefGroup := ptr.Deref(parentRef.Group, gwapiv1.Group(gwapiv1.GroupName))
				parentRefKind := ptr.Deref(parentRef.Kind, gwapiv1.Kind("Gateway"))
				if parentRefGroup != gwapiv1.GroupName || parentRefKind != "Gateway" {
					return nil, false
				}
				gatewayNamespace := string(ptr.Deref(parentRef.Namespace, gwapiv1.Namespace(httpRoute.Namespace)))
				return lo.Find(gateways, func(g *Gateway) bool {
					return g.Namespace == gatewayNamespace && g.Name == string(parentRef.Name)
				})
			})
		},
	}
}

// LinkGatewayToListenerFunc returns a link function that teaches a topology how to link gateway Listeners from the
// Gateways they are strongly related to.
func LinkGatewayToListenerFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Listener"},
		Func: func(child Object) []Object {
			listener := child.(*Listener)
			return []Object{listener.Gateway}
		},
	}
}

// LinkListenerToHTTPRouteFunc returns a link function that teaches a topology how to link HTTPRoutes from known
// Gateways and gateway Listeners, based on the HTTPRoute's `parentRefs` field.
// The function links a specific Listener of a Gateway to the HTTPRoute when the `sectionName` field of the parent
// reference is present, otherwise all Listeners of the parent Gateway are linked to the HTTPRoute.
func LinkListenerToHTTPRouteFunc(gateways []*Gateway, listeners []*Listener) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Listener"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		Func: func(child Object) []Object {
			httpRoute := child.(*HTTPRoute)
			return lo.FlatMap(httpRoute.Spec.ParentRefs, func(parentRef gwapiv1.ParentReference, _ int) []Object {
				parentRefGroup := ptr.Deref(parentRef.Group, gwapiv1.Group(gwapiv1.GroupName))
				parentRefKind := ptr.Deref(parentRef.Kind, gwapiv1.Kind("Gateway"))
				if parentRefGroup != gwapiv1.GroupName || parentRefKind != "Gateway" {
					return nil
				}
				gatewayNamespace := string(ptr.Deref(parentRef.Namespace, gwapiv1.Namespace(httpRoute.Namespace)))
				gateway, ok := lo.Find(gateways, func(g *Gateway) bool {
					return g.Namespace == gatewayNamespace && g.Name == string(parentRef.Name)
				})
				if !ok {
					return nil
				}
				if parentRef.SectionName != nil {
					listener, ok := lo.Find(listeners, func(l *Listener) bool {
						return l.Gateway.GetURL() == gateway.GetURL() && l.Name == *parentRef.SectionName
					})
					if !ok {
						return nil
					}
					return []Object{listener}
				}
				return lo.FilterMap(listeners, func(l *Listener, _ int) (Object, bool) {
					return l, l.Gateway.GetURL() == gateway.GetURL()
				})
			})
		},
	}
}

// LinkHTTPRouteToHTTPRouteRuleFunc returns a link function that teaches a topology how to link HTTPRouteRules from the
// HTTPRoute they are strongly related to.
func LinkHTTPRouteToHTTPRouteRuleFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		Func: func(child Object) []Object {
			httpRouteRule := child.(*HTTPRouteRule)
			return []Object{httpRouteRule.HTTPRoute}
		},
	}
}

// LinkHTTPRouteToServiceFunc returns a link function that teaches a topology how to link Services from known
// HTTPRoutes, based on the HTTPRoute's `backendRefs` fields.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkHTTPRouteToServiceFunc(httpRoutes []*HTTPRoute, strict bool) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		To:   schema.GroupKind{Kind: "Service"},
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(httpRoutes, func(httpRoute *HTTPRoute, _ int) (Object, bool) {
				return httpRoute, lo.ContainsBy(httpRoute.Spec.Rules, func(rule gwapiv1.HTTPRouteRule) bool {
					backendRefs := lo.FilterMap(rule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) (gwapiv1.BackendRef, bool) {
						return backendRef.BackendRef, !strict || backendRef.Port == nil
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, httpRoute.Namespace))
				})
			})
		},
	}
}

// LinkHTTPRouteToServicePortFunc returns a link function that teaches a topology how to link services ports from known
// HTTPRoutes, based on the HTTPRoute's `backendRefs` fields.
// The link function disregards backend references that do not specify a port number.
func LinkHTTPRouteToServicePortFunc(httpRoutes []*HTTPRoute) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		To:   schema.GroupKind{Kind: "ServicePort"},
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(httpRoutes, func(httpRoute *HTTPRoute, _ int) (Object, bool) {
				return httpRoute, lo.ContainsBy(httpRoute.Spec.Rules, func(rule gwapiv1.HTTPRouteRule) bool {
					backendRefs := lo.FilterMap(rule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) (gwapiv1.BackendRef, bool) {
						return backendRef.BackendRef, backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, httpRoute.Namespace))
				})
			})
		},
	}
}

// LinkHTTPRouteRuleToServiceFunc returns a link function that teaches a topology how to link Services from known
// HTTPRouteRules, based on the HTTPRouteRule's `backendRefs` field.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkHTTPRouteRuleToServiceFunc(httpRouteRules []*HTTPRouteRule, strict bool) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		To:   schema.GroupKind{Kind: "Service"},
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(httpRouteRules, func(httpRouteRule *HTTPRouteRule, _ int) (Object, bool) {
				backendRefs := lo.FilterMap(httpRouteRule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) (gwapiv1.BackendRef, bool) {
					return backendRef.BackendRef, !strict || backendRef.Port == nil
				})
				return httpRouteRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, httpRouteRule.HTTPRoute.Namespace))
			})
		},
	}
}

// LinkHTTPRouteRuleToServicePortFunc returns a link function that teaches a topology how to link services ports from
// known HTTPRouteRules, based on the HTTPRouteRule's `backendRefs` field.
// The link function disregards backend references that do not specify a port number.
func LinkHTTPRouteRuleToServicePortFunc(httpRouteRules []*HTTPRouteRule) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		To:   schema.GroupKind{Kind: "ServicePort"},
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(httpRouteRules, func(httpRouteRule *HTTPRouteRule, _ int) (Object, bool) {
				backendRefs := lo.FilterMap(httpRouteRule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) (gwapiv1.BackendRef, bool) {
					return backendRef.BackendRef, backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
				})
				return httpRouteRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, httpRouteRule.HTTPRoute.Namespace))
			})
		},
	}
}

// LinkServiceToServicePortFunc returns a link function that teaches a topology how to link service ports from the
// Serviceg they are strongly related to.
func LinkServiceToServicePortFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Kind: "Service"},
		To:   schema.GroupKind{Kind: "ServicePort"},
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return []Object{servicePort.Service}
		},
	}
}

func backendRefContainsServiceFunc(service *Service, defaultNamespace string) func(backendRef gwapiv1.BackendRef) bool {
	return func(backendRef gwapiv1.BackendRef) bool {
		return backendRefEqualToService(backendRef, service, defaultNamespace)
	}
}

func backendRefEqualToService(backendRef gwapiv1.BackendRef, service *Service, defaultNamespace string) bool {
	backendRefGroup := string(ptr.Deref(backendRef.Group, gwapiv1.Group("")))
	backendRefKind := string(ptr.Deref(backendRef.Kind, gwapiv1.Kind("Service")))
	backendRefNamespace := string(ptr.Deref(backendRef.Namespace, gwapiv1.Namespace(defaultNamespace)))
	return backendRefGroup == service.GroupVersionKind().Group && backendRefKind == service.GroupVersionKind().Kind && backendRefNamespace == service.Namespace && string(backendRef.Name) == service.Name
}
