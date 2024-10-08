package machinery

import (
	"fmt"

	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type GatewayAPITopologyOptions struct {
	GatewayClasses []*GatewayClass
	Gateways       []*Gateway
	HTTPRoutes     []*HTTPRoute
	GRPCRoutes     []*GRPCRoute
	TCPRoutes      []*TCPRoute
	TLSRoutes      []*TLSRoute
	UDPRoutes      []*UDPRoute
	Services       []*Service
	Policies       []Policy
	Objects        []Object
	Links          []LinkFunc

	ExpandGatewayListeners bool
	ExpandHTTPRouteRules   bool
	ExpandGRPCRouteRules   bool
	ExpandTCPRouteRules    bool
	ExpandTLSRouteRules    bool
	ExpandUDPRouteRules    bool
	ExpandServicePorts     bool

	allowTopologyLoops bool
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

// WithGRPCRoutes adds GRPC routes to the options to initialize a new Gateway API topology.
func WithGRPCRoutes(grpcRoutes ...*gwapiv1.GRPCRoute) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.GRPCRoutes = append(o.GRPCRoutes, lo.Map(grpcRoutes, func(grpcRoute *gwapiv1.GRPCRoute, _ int) *GRPCRoute {
			return &GRPCRoute{GRPCRoute: grpcRoute}
		})...)
	}
}

// WithTCPRoutes adds TCP routes to the options to initialize a new Gateway API topology.
func WithTCPRoutes(tcpRoutes ...*gwapiv1alpha2.TCPRoute) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.TCPRoutes = append(o.TCPRoutes, lo.Map(tcpRoutes, func(tcpRoute *gwapiv1alpha2.TCPRoute, _ int) *TCPRoute {
			return &TCPRoute{TCPRoute: tcpRoute}
		})...)
	}
}

// WithTLSRoutes adds TLS routes to the options to initialize a new Gateway API topology.
func WithTLSRoutes(tlsRoutes ...*gwapiv1alpha2.TLSRoute) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.TLSRoutes = append(o.TLSRoutes, lo.Map(tlsRoutes, func(tlsRoute *gwapiv1alpha2.TLSRoute, _ int) *TLSRoute {
			return &TLSRoute{TLSRoute: tlsRoute}
		})...)
	}
}

// WithUDPRoutes adds UDP routes to the options to initialize a new Gateway API topology.
func WithUDPRoutes(udpRoutes ...*gwapiv1alpha2.UDPRoute) GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.UDPRoutes = append(o.UDPRoutes, lo.Map(udpRoutes, func(udpRoute *gwapiv1alpha2.UDPRoute, _ int) *UDPRoute {
			return &UDPRoute{UDPRoute: udpRoute}
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

// WithGatewayAPITopologyLinks adds link functions to the options to initialize a new Gateway API topology.
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

// ExpandGRPCRouteRules adds targetable GRPC route rules to the options to initialize a new Gateway API topology.
func ExpandGRPCRouteRules() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.ExpandGRPCRouteRules = true
	}
}

// ExpandTCPRouteRules adds targetable TCP route rules to the options to initialize a new Gateway API topology.
func ExpandTCPRouteRules() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.ExpandTCPRouteRules = true
	}
}

// ExpandTLSRouteRules adds targetable TLS route rules to the options to initialize a new Gateway API topology.
func ExpandTLSRouteRules() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.ExpandTLSRouteRules = true
	}
}

// ExpandUDPRouteRules adds targetable UDP route rules to the options to initialize a new Gateway API topology.
func ExpandUDPRouteRules() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.ExpandUDPRouteRules = true
	}
}

// ExpandServicePorts adds targetable service ports to the options to initialize a new Gateway API topology.
func ExpandServicePorts() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.ExpandServicePorts = true
	}
}

// AllowTopologyLoops adds AllowLoops to the options to initialize a new Gateway API topology.
func AllowTopologyLoops() GatewayAPITopologyOptionsFunc {
	return func(o *GatewayAPITopologyOptions) {
		o.allowTopologyLoops = true
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
func NewGatewayAPITopology(options ...GatewayAPITopologyOptionsFunc) (*Topology, error) {
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
		WithTargetables(o.GRPCRoutes...),
		WithTargetables(o.TCPRoutes...),
		WithTargetables(o.TLSRoutes...),
		WithTargetables(o.UDPRoutes...),
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
			LinkListenerToGRPCRouteFunc(o.Gateways, listeners), // Listener -> GRPCRoute
			LinkListenerToTCPRouteFunc(o.Gateways, listeners),  // Listener -> TCPRoute
			LinkListenerToTLSRouteFunc(o.Gateways, listeners),  // Listener -> TLSRoute
			LinkListenerToUDPRouteFunc(o.Gateways, listeners),  // Listener -> UDPRoute
		))
	} else {
		opts = append(opts, WithLinks(
			LinkGatewayToHTTPRouteFunc(o.Gateways), // Gateway -> HTTPRoute
			LinkGatewayToGRPCRouteFunc(o.Gateways), // Gateway -> GRPCRoute
			LinkGatewayToTCPRouteFunc(o.Gateways),  // Gateway -> TCPRoute
			LinkGatewayToTLSRouteFunc(o.Gateways),  // Gateway -> TLSRoute
			LinkGatewayToUDPRouteFunc(o.Gateways),  // Gateway -> UDPRoute
		))
	}

	if o.ExpandHTTPRouteRules {
		httpRouteRules := lo.FlatMap(o.HTTPRoutes, HTTPRouteRulesFromHTTPRouteFunc)
		opts = append(opts, WithTargetables(httpRouteRules...))
		opts = append(opts, WithLinks(LinkHTTPRouteToHTTPRouteRuleFunc())) // HTTPRoute -> HTTPRouteRule

		if o.ExpandServicePorts {
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

	if o.ExpandGRPCRouteRules {
		grpcRouteRules := lo.FlatMap(o.GRPCRoutes, GRPCRouteRulesFromGRPCRouteRule)
		opts = append(opts, WithTargetables(grpcRouteRules...))
		opts = append(opts, WithLinks(LinkGRPCRouteToGRPCRouteRuleFunc())) // GRPCRoute -> GRPCRouteRule

		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkGRPCRouteRuleToServicePortFunc(grpcRouteRules),   // GRPCRouteRule -> ServicePort
				LinkGRPCRouteRuleToServiceFunc(grpcRouteRules, true), // GRPCRouteRule -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkGRPCRouteRuleToServiceFunc(grpcRouteRules, false))) // GRPCRouteRule -> Service
		}
	} else {
		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkGRPCRouteToServicePortFunc(o.GRPCRoutes),   // GRPCRoute -> ServicePort
				LinkGRPCRouteToServiceFunc(o.GRPCRoutes, true), // GRPCRoute -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkGRPCRouteToServiceFunc(o.GRPCRoutes, false))) // GRPCRoute -> Service
		}
	}

	if o.ExpandTCPRouteRules {
		tcpRouteRules := lo.FlatMap(o.TCPRoutes, TCPRouteRulesFromTCPRouteFunc)
		opts = append(opts, WithTargetables(tcpRouteRules...))
		opts = append(opts, WithLinks(LinkTCPRouteToTCPRouteRuleFunc())) // TCPRoute - TCPRouteRules

		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkTCPRouteRuleToServicePortFunc(tcpRouteRules),   // TCPRouteRule -> ServicePort
				LinkTCPRouteRuleToServiceFunc(tcpRouteRules, true), // TCPRoute -> service
			))
		} else {
			opts = append(opts, WithLinks(LinkTCPRouteRuleToServiceFunc(tcpRouteRules, false))) // TCPRouteRule -> Service
		}
	} else {
		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkTCPRouteToServicePortFunc(o.TCPRoutes),   // TCPRoute -> ServicePort
				LinkTCPRouteToServiceFunc(o.TCPRoutes, true), // TCPRoute -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkTCPRouteToServiceFunc(o.TCPRoutes, false))) // TCPRoute -> Service
		}
	}

	if o.ExpandTLSRouteRules {
		tlsRouteRules := lo.FlatMap(o.TLSRoutes, TLSRouteRulesFromTLSRouteFunc)
		opts = append(opts, WithTargetables(tlsRouteRules...))
		opts = append(opts, WithLinks(LinkTLSRouteToTLSRouteRuleFunc()))

		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkTLSRouteRuleToServicePortFunc(tlsRouteRules),   // TLSRouteRule -> ServicePort
				LinkTLSRouteRuleToServiceFunc(tlsRouteRules, true), // TLSRouteRule -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkTLSRouteToServiceFunc(o.TLSRoutes, false))) // TLSRoute -> Service
		}
	} else {
		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkTLSRouteToServicePortFunc(o.TLSRoutes),   // TLSRoute -> ServicePort
				LinkTLSRouteToServiceFunc(o.TLSRoutes, true), // TLSRoute -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkTLSRouteToServiceFunc(o.TLSRoutes, false))) // TLSRoute -> Service
		}
	}

	if o.ExpandUDPRouteRules {
		udpRouteRules := lo.FlatMap(o.UDPRoutes, UDPRouteRulesFromUDPRouteFunc)
		opts = append(opts, WithTargetables(udpRouteRules...))
		opts = append(opts, WithLinks(LinkUDPRouteToUDPRouteRuleFunc()))

		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkUDPRouteRuleToServicePortFunc(udpRouteRules),   // UDPRouteRule -> ServicePort
				LinkUDPRouteRuleToServiceFunc(udpRouteRules, true), // UDPRouteRule -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkUDPRouteToServiceFunc(o.UDPRoutes, false))) // UDPRoute -> Service
		}
	} else {
		if o.ExpandServicePorts {
			opts = append(opts, WithLinks(
				LinkUDPRouteToServicePortFunc(o.UDPRoutes),   // UDPRoute -> ServicePort
				LinkUDPRouteToServiceFunc(o.UDPRoutes, true), // UDPRoute -> Service
			))
		} else {
			opts = append(opts, WithLinks(LinkUDPRouteToServiceFunc(o.UDPRoutes, false))) // UDPRoute -> Service
		}
	}

	if o.ExpandServicePorts {
		servicePorts := lo.FlatMap(o.Services, ServicePortsFromServiceFunc)
		opts = append(opts, WithTargetables(servicePorts...))
		opts = append(opts, WithLinks(LinkServiceToServicePortFunc())) // Service -> ServicePort
	}

	if o.allowTopologyLoops {
		opts = append(opts, AllowLoops())
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

// GRPCRouteRulesFromGRPCRouteRule returns a list of targetable GRPCRouteRules from a targetable GRPCRoute.
func GRPCRouteRulesFromGRPCRouteRule(grpcRoute *GRPCRoute, _ int) []*GRPCRouteRule {
	return lo.Map(grpcRoute.Spec.Rules, func(rule gwapiv1.GRPCRouteRule, i int) *GRPCRouteRule {
		return &GRPCRouteRule{
			GRPCRouteRule: &rule,
			GRPCRoute:     grpcRoute,
			Name:          gwapiv1.SectionName(fmt.Sprintf("rule-%d", i+1)),
		}
	})
}

// TCPRouteRulesFromTCPRouteFunc returns a list of targetable TCPRouteRules from a targetable TCPRoute.
func TCPRouteRulesFromTCPRouteFunc(tcpRoute *TCPRoute, _ int) []*TCPRouteRule {
	return lo.Map(tcpRoute.Spec.Rules, func(rule gwapiv1alpha2.TCPRouteRule, i int) *TCPRouteRule {
		return &TCPRouteRule{
			TCPRouteRule: &rule,
			TCPRoute:     tcpRoute,
			Name:         gwapiv1.SectionName(fmt.Sprintf("rule-%d", i+1)),
		}
	})
}

// TLSRouteRulesFromTLSRouteFunc returns a list of targetable TCPRouteRules from a targetable TLSRoute.
func TLSRouteRulesFromTLSRouteFunc(tlsRoute *TLSRoute, _ int) []*TLSRouteRule {
	return lo.Map(tlsRoute.Spec.Rules, func(rule gwapiv1alpha2.TLSRouteRule, i int) *TLSRouteRule {
		return &TLSRouteRule{
			TLSRouteRule: &rule,
			TLSRoute:     tlsRoute,
			Name:         gwapiv1.SectionName(fmt.Sprintf("rule-%d", i+1)),
		}
	})
}

// UDPRouteRulesFromUDPRouteFunc returns a list of targetable UDPRouteRules from a targetable UDPRoute.
func UDPRouteRulesFromUDPRouteFunc(udpRoute *UDPRoute, _ int) []*UDPRouteRule {
	return lo.Map(udpRoute.Spec.Rules, func(rule gwapiv1alpha2.UDPRouteRule, i int) *UDPRouteRule {
		return &UDPRouteRule{
			UDPRouteRule: &rule,
			UDPRoute:     udpRoute,
			Name:         gwapiv1.SectionName(fmt.Sprintf("rule-%d", i+1)),
		}
	})
}

// ServicePortsFromServiceFunc returns a list of targetable service ports from a targetable Service.
func ServicePortsFromServiceFunc(service *Service, _ int) []*ServicePort {
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
		From: GatewayClassGroupKind,
		To:   GatewayGroupKind,
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
		From: GatewayGroupKind,
		To:   HTTPRouteGroupKind,
		Func: func(child Object) []Object {
			httpRoute := child.(*HTTPRoute)
			return lo.FilterMap(httpRoute.Spec.ParentRefs, findGatewayFromParentRefFunc(gateways, httpRoute.Namespace))
		},
	}
}

// LinkGatewayToGRPCRouteFunc returns a link function that teaches a topology how to link GRPCRoute's from known
// Gateway's, based on the GRPCRoute's `parentRefs` field.
func LinkGatewayToGRPCRouteFunc(gateways []*Gateway) LinkFunc {
	return LinkFunc{
		From: GatewayGroupKind,
		To:   GRPCRouteGroupKind,
		Func: func(child Object) []Object {
			grpcRoute := child.(*GRPCRoute)
			return lo.FilterMap(grpcRoute.Spec.ParentRefs, findGatewayFromParentRefFunc(gateways, grpcRoute.Namespace))
		},
	}
}

// LinkGatewayToTCPRouteFunc returns a link function that teaches a topology how to link TCPRoute's from known
// Gateway's, based on the TCPRoute's `parentRefs` field.
func LinkGatewayToTCPRouteFunc(gateways []*Gateway) LinkFunc {
	return LinkFunc{
		From: GatewayGroupKind,
		To:   TCPRouteGroupKind,
		Func: func(child Object) []Object {
			tcpRoute := child.(*TCPRoute)
			return lo.FilterMap(tcpRoute.Spec.ParentRefs, findGatewayFromParentRefFunc(gateways, tcpRoute.Namespace))
		},
	}
}

// LinkGatewayToTLSRouteFunc returns a link function that teaches a topology how to link TLSRoute's from known
// Gateway's, based on the TLSRoute's `parentRefs` field.
func LinkGatewayToTLSRouteFunc(gateways []*Gateway) LinkFunc {
	return LinkFunc{
		From: GatewayGroupKind,
		To:   TLSRouteGroupKind,
		Func: func(child Object) []Object {
			tlsRoute := child.(*TLSRoute)
			return lo.FilterMap(tlsRoute.Spec.ParentRefs, findGatewayFromParentRefFunc(gateways, tlsRoute.Namespace))
		},
	}
}

// LinkGatewayToUDPRouteFunc returns a link function that teaches a topology how to link UDPRoute's from known
// Gateway's, based on the UDPRoute's `parentRefs` field.
func LinkGatewayToUDPRouteFunc(gateways []*Gateway) LinkFunc {
	return LinkFunc{
		From: GatewayGroupKind,
		To:   UDPRouteGroupKind,
		Func: func(child Object) []Object {
			udpRoute := child.(*UDPRoute)
			return lo.FilterMap(udpRoute.Spec.ParentRefs, findGatewayFromParentRefFunc(gateways, udpRoute.Namespace))
		},
	}
}

// findGatewayFromParentRefFunc is a common function to find a Gateway from a xRoute's `parentRef` field
func findGatewayFromParentRefFunc(gateways []*Gateway, routeNamespace string) func(parentRef gwapiv1.ParentReference, _ int) (Object, bool) {
	return func(parentRef gwapiv1.ParentReference, _ int) (Object, bool) {
		parentRefGroup := ptr.Deref(parentRef.Group, gwapiv1.GroupName)
		parentRefKind := ptr.Deref(parentRef.Kind, "Gateway")
		if parentRefGroup != gwapiv1.GroupName || parentRefKind != "Gateway" {
			return nil, false
		}
		gatewayNamespace := string(ptr.Deref(parentRef.Namespace, gwapiv1.Namespace(routeNamespace)))
		return lo.Find(gateways, func(g *Gateway) bool {
			return g.Namespace == gatewayNamespace && g.Name == string(parentRef.Name)
		})
	}
}

// LinkGatewayToListenerFunc returns a link function that teaches a topology how to link gateway Listeners from the
// Gateways they are strongly related to.
func LinkGatewayToListenerFunc() LinkFunc {
	return LinkFunc{
		From: GatewayGroupKind,
		To:   ListenerGroupKind,
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
		From: ListenerGroupKind,
		To:   HTTPRouteGroupKind,
		Func: func(child Object) []Object {
			httpRoute := child.(*HTTPRoute)
			return lo.FlatMap(httpRoute.Spec.ParentRefs, findListenerFromParentRefFunc(gateways, listeners, httpRoute.Namespace))
		},
	}
}

// LinkListenerToGRPCRouteFunc returns a link function that teaches a topology how to link GRPCRoutes from known
// Gateways and gateway Listeners, based on the GRPCRoute's `parentRefs` field.
// The function links a specific Listener of a Gateway to the GRPCRoute when the `sectionName` field of the parent
// reference is present, otherwise all Listeners of the parent Gateway are linked to the GRPCRoute.
func LinkListenerToGRPCRouteFunc(gateways []*Gateway, listeners []*Listener) LinkFunc {
	return LinkFunc{
		From: ListenerGroupKind,
		To:   GRPCRouteGroupKind,
		Func: func(child Object) []Object {
			grpcRoute := child.(*GRPCRoute)
			return lo.FlatMap(grpcRoute.Spec.ParentRefs, findListenerFromParentRefFunc(gateways, listeners, grpcRoute.Namespace))
		},
	}
}

// LinkListenerToTCPRouteFunc returns a link function that teaches a topology how to link TCPRoutes from known
// Gateways and gateway Listeners, based on the TCPRoute's `parentRefs` field.
// The function links a specific Listener of a Gateway to the TCPRoute when the `sectionName` field of the parent
// reference is present, otherwise all Listeners of the parent Gateway are linked to the TCPRoute.
func LinkListenerToTCPRouteFunc(gateways []*Gateway, listeners []*Listener) LinkFunc {
	return LinkFunc{
		From: ListenerGroupKind,
		To:   TCPRouteGroupKind,
		Func: func(child Object) []Object {
			tcpRoute := child.(*TCPRoute)
			return lo.FlatMap(tcpRoute.Spec.ParentRefs, findListenerFromParentRefFunc(gateways, listeners, tcpRoute.Namespace))
		},
	}
}

// LinkListenerToTLSRouteFunc returns a link function that teaches a topology how to link TLSRoutes from known
// Gateways and gateway Listeners, based on the TLSRoute's `parentRefs` field.
// The function links a specific Listener of a Gateway to the TLSRoute when the `sectionName` field of the parent
// reference is present, otherwise all Listeners of the parent Gateway are linked to the TLSRoute.
func LinkListenerToTLSRouteFunc(gateways []*Gateway, listeners []*Listener) LinkFunc {
	return LinkFunc{
		From: ListenerGroupKind,
		To:   TLSRouteGroupKind,
		Func: func(child Object) []Object {
			tlsRoute := child.(*TLSRoute)
			return lo.FlatMap(tlsRoute.Spec.ParentRefs, findListenerFromParentRefFunc(gateways, listeners, tlsRoute.Namespace))
		},
	}
}

// LinkListenerToUDPRouteFunc returns a link function that teaches a topology how to link UDPRoutes from known
// Gateways and gateway Listeners, based on the UDPRoute's `parentRefs` field.
// The function links a specific Listener of a Gateway to the UDPRoute when the `sectionName` field of the parent
// reference is present, otherwise all Listeners of the parent Gateway are linked to the UDPRoute.
func LinkListenerToUDPRouteFunc(gateways []*Gateway, listeners []*Listener) LinkFunc {
	return LinkFunc{
		From: ListenerGroupKind,
		To:   UDPRouteGroupKind,
		Func: func(child Object) []Object {
			udpRoute := child.(*UDPRoute)
			return lo.FlatMap(udpRoute.Spec.ParentRefs, findListenerFromParentRefFunc(gateways, listeners, udpRoute.Namespace))
		},
	}
}

// findListenerFromParentRefFunc is a common function to find a gateway Listener from a xRoute's `parentRef` field
func findListenerFromParentRefFunc(gateways []*Gateway, listeners []*Listener, routeNamespace string) func(parentRef gwapiv1.ParentReference, _ int) []Object {
	return func(parentRef gwapiv1.ParentReference, _ int) []Object {
		parentRefGroup := ptr.Deref(parentRef.Group, gwapiv1.GroupName)
		parentRefKind := ptr.Deref(parentRef.Kind, "Gateway")
		if parentRefGroup != gwapiv1.GroupName || parentRefKind != "Gateway" {
			return nil
		}
		gatewayNamespace := string(ptr.Deref(parentRef.Namespace, gwapiv1.Namespace(routeNamespace)))
		gateway, ok := lo.Find(gateways, func(g *Gateway) bool {
			return g.Namespace == gatewayNamespace && g.Name == string(parentRef.Name)
		})
		if !ok {
			return nil
		}
		if parentRef.SectionName != nil {
			listener, ok := lo.Find(listeners, func(l *Listener) bool {
				return l.Gateway.GetLocator() == gateway.GetLocator() && l.Name == *parentRef.SectionName
			})
			if !ok {
				return nil
			}
			return []Object{listener}
		}
		return lo.FilterMap(listeners, func(l *Listener, _ int) (Object, bool) {
			return l, l.Gateway.GetLocator() == gateway.GetLocator()
		})
	}
}

// LinkHTTPRouteToHTTPRouteRuleFunc returns a link function that teaches a topology how to link HTTPRouteRules from the
// HTTPRoute they are strongly related to.
func LinkHTTPRouteToHTTPRouteRuleFunc() LinkFunc {
	return LinkFunc{
		From: HTTPRouteGroupKind,
		To:   HTTPRouteRuleGroupKind,
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
		From: HTTPRouteGroupKind,
		To:   ServiceGroupKind,
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
		From: HTTPRouteGroupKind,
		To:   ServicePortGroupKind,
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
		From: HTTPRouteRuleGroupKind,
		To:   ServiceGroupKind,
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
		From: HTTPRouteRuleGroupKind,
		To:   ServicePortGroupKind,
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

// LinkGRPCRouteToServiceFunc returns a link function that teaches a topology how to link Services from known
// GRPCRoutes, based on the GRPCRoute's `backendRefs` fields.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkGRPCRouteToServiceFunc(routes []*GRPCRoute, strict bool) LinkFunc {
	return LinkFunc{
		From: GRPCRouteGroupKind,
		To:   ServiceGroupKind,
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(routes, func(route *GRPCRoute, _ int) (Object, bool) {
				return route, lo.ContainsBy(route.Spec.Rules, func(rule gwapiv1.GRPCRouteRule) bool {
					backendRefs := lo.FilterMap(rule.BackendRefs, func(backendRef gwapiv1.GRPCBackendRef, _ int) (gwapiv1.BackendRef, bool) {
						return backendRef.BackendRef, !strict || backendRef.Port == nil
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, route.Namespace))
				})
			})
		},
	}
}

// LinkGRPCRouteToServicePortFunc returns a link function that teaches a topology how to link services ports from known
// GRPCRoutes, based on the GRPCRoute's `backendRefs` fields.
// The link function disregards backend references that do not specify a port number.
func LinkGRPCRouteToServicePortFunc(routes []*GRPCRoute) LinkFunc {
	return LinkFunc{
		From: GRPCRouteGroupKind,
		To:   ServicePortGroupKind,
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(routes, func(route *GRPCRoute, _ int) (Object, bool) {
				return route, lo.ContainsBy(route.Spec.Rules, func(rule gwapiv1.GRPCRouteRule) bool {
					backendRefs := lo.FilterMap(rule.BackendRefs, func(backendRef gwapiv1.GRPCBackendRef, _ int) (gwapiv1.BackendRef, bool) {
						return backendRef.BackendRef, backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, route.Namespace))
				})
			})
		},
	}
}

// LinkGRPCRouteToGRPCRouteRuleFunc returns a link function that teaches a topology how to link GRPCRouteRule from the
// GRPCRoute they are strongly related to.
func LinkGRPCRouteToGRPCRouteRuleFunc() LinkFunc {
	return LinkFunc{
		From: GRPCRouteGroupKind,
		To:   GRPCRouteRuleGroupKind,
		Func: func(child Object) []Object {
			grpcRouteRule := child.(*GRPCRouteRule)
			return []Object{grpcRouteRule.GRPCRoute}
		},
	}
}

// LinkGRPCRouteRuleToServiceFunc returns a link function that teaches a topology how to link Services from known
// GRPCRouteRules, based on the GRPCRouteRule's `backendRefs` field.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkGRPCRouteRuleToServiceFunc(routeRules []*GRPCRouteRule, strict bool) LinkFunc {
	return LinkFunc{
		From: GRPCRouteRuleGroupKind,
		To:   ServiceGroupKind,
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(routeRules, func(routeRule *GRPCRouteRule, _ int) (Object, bool) {
				backendRefs := lo.FilterMap(routeRule.BackendRefs, func(backendRef gwapiv1.GRPCBackendRef, _ int) (gwapiv1.BackendRef, bool) {
					return backendRef.BackendRef, !strict || backendRef.Port == nil
				})
				return routeRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, routeRule.GRPCRoute.Namespace))
			})
		},
	}
}

// LinkGRPCRouteRuleToServicePortFunc returns a link function that teaches a topology how to link services ports from
// known GRPCRouteRules, based on the GRPCRouteRule's `backendRefs` field.
// The link function disregards backend references that do not specify a port number.
func LinkGRPCRouteRuleToServicePortFunc(routeRules []*GRPCRouteRule) LinkFunc {
	return LinkFunc{
		From: GRPCRouteRuleGroupKind,
		To:   ServicePortGroupKind,
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(routeRules, func(routeRule *GRPCRouteRule, _ int) (Object, bool) {
				backendRefs := lo.FilterMap(routeRule.BackendRefs, func(backendRef gwapiv1.GRPCBackendRef, _ int) (gwapiv1.BackendRef, bool) {
					return backendRef.BackendRef, backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
				})
				return routeRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, routeRule.GRPCRoute.Namespace))
			})
		},
	}
}

// LinkTCPRouteToServiceFunc returns a link function that teaches a topology how to link Services from known
// GRPCRoutes, based on the TCPRoute's `backendRefs` fields.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkTCPRouteToServiceFunc(routes []*TCPRoute, strict bool) LinkFunc {
	return LinkFunc{
		From: TCPRouteGroupKind,
		To:   ServiceGroupKind,
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(routes, func(route *TCPRoute, _ int) (Object, bool) {
				return route, lo.ContainsBy(route.Spec.Rules, func(rule gwapiv1alpha2.TCPRouteRule) bool {
					backendRefs := lo.Filter(rule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
						return !strict || backendRef.Port == nil
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, route.Namespace))
				})
			})
		},
	}
}

// LinkTCPRouteToServicePortFunc returns a link function that teaches a topology how to link services ports from known
// TCPRoutes, based on the TCPRoute's `backendRefs` fields.
// The link function disregards backend references that do not specify a port number.
func LinkTCPRouteToServicePortFunc(routes []*TCPRoute) LinkFunc {
	return LinkFunc{
		From: TCPRouteGroupKind,
		To:   ServicePortGroupKind,
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(routes, func(route *TCPRoute, _ int) (Object, bool) {
				return route, lo.ContainsBy(route.Spec.Rules, func(rule gwapiv1alpha2.TCPRouteRule) bool {
					backendRefs := lo.Filter(rule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
						return backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, route.Namespace))
				})
			})
		},
	}
}

// LinkTCPRouteToTCPRouteRuleFunc returns a link function that teaches a topology how to link TCPRouteRule from the
// TCPRoute they are strongly related to.
func LinkTCPRouteToTCPRouteRuleFunc() LinkFunc {
	return LinkFunc{
		From: TCPRouteGroupKind,
		To:   TCPRouteRuleGroupKind,
		Func: func(child Object) []Object {
			tcpRouteRule := child.(*TCPRouteRule)
			return []Object{tcpRouteRule.TCPRoute}
		},
	}
}

// LinkTCPRouteRuleToServiceFunc returns a link function that teaches a topology how to link Services from known
// TCPRouteRules, based on the TCPRouteRule's `backendRefs` field.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkTCPRouteRuleToServiceFunc(routeRules []*TCPRouteRule, strict bool) LinkFunc {
	return LinkFunc{
		From: TCPRouteRuleGroupKind,
		To:   ServiceGroupKind,
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(routeRules, func(routeRule *TCPRouteRule, _ int) (Object, bool) {
				backendRefs := lo.Filter(routeRule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
					return !strict || backendRef.Port == nil
				})
				return routeRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, routeRule.TCPRoute.Namespace))
			})
		},
	}
}

// LinkTCPRouteRuleToServicePortFunc returns a link function that teaches a topology how to link services ports from
// known TCPRouteRules, based on the TCPRouteRule's `backendRefs` field.
// The link function disregards backend references that do not specify a port number.
func LinkTCPRouteRuleToServicePortFunc(routeRules []*TCPRouteRule) LinkFunc {
	return LinkFunc{
		From: TCPRouteRuleGroupKind,
		To:   ServicePortGroupKind,
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(routeRules, func(routeRule *TCPRouteRule, _ int) (Object, bool) {
				backendRefs := lo.Filter(routeRule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
					return backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
				})
				return routeRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, routeRule.TCPRoute.Namespace))
			})
		},
	}
}

// LinkTLSRouteToServiceFunc returns a link function that teaches a topology how to link Services from known
// TLSRoutes, based on the TLSRoute's `backendRefs` fields.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkTLSRouteToServiceFunc(routes []*TLSRoute, strict bool) LinkFunc {
	return LinkFunc{
		From: TLSRouteGroupKind,
		To:   ServiceGroupKind,
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(routes, func(route *TLSRoute, _ int) (Object, bool) {
				return route, lo.ContainsBy(route.Spec.Rules, func(rule gwapiv1alpha2.TLSRouteRule) bool {
					backendRefs := lo.Filter(rule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
						return !strict || backendRef.Port == nil
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, route.Namespace))
				})
			})
		},
	}
}

// LinkTLSRouteToServicePortFunc returns a link function that teaches a topology how to link services ports from known
// TLSRoutes, based on the TLSRoute's `backendRefs` fields.
// The link function disregards backend references that do not specify a port number.
func LinkTLSRouteToServicePortFunc(routes []*TLSRoute) LinkFunc {
	return LinkFunc{
		From: TLSRouteGroupKind,
		To:   ServicePortGroupKind,
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(routes, func(route *TLSRoute, _ int) (Object, bool) {
				return route, lo.ContainsBy(route.Spec.Rules, func(rule gwapiv1alpha2.TLSRouteRule) bool {
					backendRefs := lo.Filter(rule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
						return backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, route.Namespace))
				})
			})
		},
	}
}

// LinkTLSRouteToTLSRouteRuleFunc returns a link function that teaches a topology how to link TLSRouteRule from the
// TLSRoute they are strongly related to.
func LinkTLSRouteToTLSRouteRuleFunc() LinkFunc {
	return LinkFunc{
		From: TLSRouteGroupKind,
		To:   TLSRouteRuleGroupKind,
		Func: func(child Object) []Object {
			tlsRouteRule := child.(*TLSRouteRule)
			return []Object{tlsRouteRule.TLSRoute}
		},
	}
}

// LinkTLSRouteRuleToServiceFunc returns a link function that teaches a topology how to link Services from known
// TLSRouteRules, based on the TLSRouteRule's `backendRefs` field.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkTLSRouteRuleToServiceFunc(routeRules []*TLSRouteRule, strict bool) LinkFunc {
	return LinkFunc{
		From: TLSRouteRuleGroupKind,
		To:   ServiceGroupKind,
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(routeRules, func(routeRule *TLSRouteRule, _ int) (Object, bool) {
				backendRefs := lo.Filter(routeRule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
					return !strict || backendRef.Port == nil
				})
				return routeRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, routeRule.TLSRoute.Namespace))
			})
		},
	}
}

// LinkTLSRouteRuleToServicePortFunc returns a link function that teaches a topology how to link services ports from
// known TLSRouteRules, based on the TLSRouteRule's `backendRefs` field.
// The link function disregards backend references that do not specify a port number.
func LinkTLSRouteRuleToServicePortFunc(routeRules []*TLSRouteRule) LinkFunc {
	return LinkFunc{
		From: TLSRouteRuleGroupKind,
		To:   ServicePortGroupKind,
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(routeRules, func(routeRule *TLSRouteRule, _ int) (Object, bool) {
				backendRefs := lo.Filter(routeRule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
					return backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
				})
				return routeRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, routeRule.TLSRoute.Namespace))
			})
		},
	}
}

// LinkUDPRouteToServiceFunc returns a link function that teaches a topology how to link Services from known
// UDPRoutes, based on the UDPRoute's `backendRefs` fields.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkUDPRouteToServiceFunc(routes []*UDPRoute, strict bool) LinkFunc {
	return LinkFunc{
		From: UDPRouteGroupKind,
		To:   ServiceGroupKind,
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(routes, func(route *UDPRoute, _ int) (Object, bool) {
				return route, lo.ContainsBy(route.Spec.Rules, func(rule gwapiv1alpha2.UDPRouteRule) bool {
					backendRefs := lo.Filter(rule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
						return !strict || backendRef.Port == nil
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, route.Namespace))
				})
			})
		},
	}
}

// LinkUDPRouteToServicePortFunc returns a link function that teaches a topology how to link services ports from known
// UDPRoutes, based on the UDPRoute's `backendRefs` fields.
// The link function disregards backend references that do not specify a port number.
func LinkUDPRouteToServicePortFunc(routes []*UDPRoute) LinkFunc {
	return LinkFunc{
		From: UDPRouteGroupKind,
		To:   ServicePortGroupKind,
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(routes, func(route *UDPRoute, _ int) (Object, bool) {
				return route, lo.ContainsBy(route.Spec.Rules, func(rule gwapiv1alpha2.UDPRouteRule) bool {
					backendRefs := lo.Filter(rule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
						return backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
					})
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, route.Namespace))
				})
			})
		},
	}
}

// LinkUDPRouteToUDPRouteRuleFunc returns a link function that teaches a topology how to link UDPRouteRule from the
// UDPRoute they are strongly related to.
func LinkUDPRouteToUDPRouteRuleFunc() LinkFunc {
	return LinkFunc{
		From: UDPRouteGroupKind,
		To:   UDPRouteRuleGroupKind,
		Func: func(child Object) []Object {
			updRouteRule := child.(*UDPRouteRule)
			return []Object{updRouteRule.UDPRoute}
		},
	}
}

// LinkUDPRouteRuleToServiceFunc returns a link function that teaches a topology how to link Services from known
// UDPRouteRules, based on the UDPRouteRule's `backendRefs` field.
// Set the `strict` parameter to `true` to link only to services that have no port specified in the backendRefs.
func LinkUDPRouteRuleToServiceFunc(routeRules []*UDPRouteRule, strict bool) LinkFunc {
	return LinkFunc{
		From: UDPRouteRuleGroupKind,
		To:   ServiceGroupKind,
		Func: func(child Object) []Object {
			service := child.(*Service)
			return lo.FilterMap(routeRules, func(routeRule *UDPRouteRule, _ int) (Object, bool) {
				backendRefs := lo.Filter(routeRule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
					return !strict || backendRef.Port == nil
				})
				return routeRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, routeRule.UDPRoute.Namespace))
			})
		},
	}
}

// LinkUDPRouteRuleToServicePortFunc returns a link function that teaches a topology how to link services ports from
// known UDPRouteRules, based on the UDPRouteRule's `backendRefs` field.
// The link function disregards backend references that do not specify a port number.
func LinkUDPRouteRuleToServicePortFunc(routeRules []*UDPRouteRule) LinkFunc {
	return LinkFunc{
		From: UDPRouteRuleGroupKind,
		To:   ServicePortGroupKind,
		Func: func(child Object) []Object {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(routeRules, func(routeRule *UDPRouteRule, _ int) (Object, bool) {
				backendRefs := lo.Filter(routeRule.BackendRefs, func(backendRef gwapiv1.BackendRef, _ int) bool {
					return backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
				})
				return routeRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, routeRule.UDPRoute.Namespace))
			})
		},
	}
}

// LinkServiceToServicePortFunc returns a link function that teaches a topology how to link service ports from the
// Service they are strongly related to.
func LinkServiceToServicePortFunc() LinkFunc {
	return LinkFunc{
		From: ServiceGroupKind,
		To:   ServicePortGroupKind,
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
