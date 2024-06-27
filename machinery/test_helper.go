//go:build unit || integration

package machinery

import (
	"fmt"

	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func BuildGatewayClass(f ...func(*gwapiv1.GatewayClass)) *gwapiv1.GatewayClass {
	gc := &gwapiv1.GatewayClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gwapiv1.GroupVersion.String(),
			Kind:       "GatewayClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-gateway-class",
		},
		Spec: gwapiv1.GatewayClassSpec{
			ControllerName: gwapiv1.GatewayController("my-gateway-controller"),
		},
	}
	for _, fn := range f {
		fn(gc)
	}
	return gc
}

func BuildGateway(f ...func(*gwapiv1.Gateway)) *gwapiv1.Gateway {
	g := &gwapiv1.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gwapiv1.GroupVersion.String(),
			Kind:       "Gateway",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-gateway",
			Namespace: "my-namespace",
		},
		Spec: gwapiv1.GatewaySpec{
			GatewayClassName: "my-gateway-class",
			Listeners: []gwapiv1.Listener{
				{
					Name:     "my-listener",
					Port:     80,
					Protocol: "HTTP",
				},
			},
		},
	}
	for _, fn := range f {
		fn(g)
	}
	return g
}

func BuildHTTPRoute(f ...func(*gwapiv1.HTTPRoute)) *gwapiv1.HTTPRoute {
	r := &gwapiv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gwapiv1.GroupVersion.String(),
			Kind:       "HTTPRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-http-route",
			Namespace: "my-namespace",
		},
		Spec: gwapiv1.HTTPRouteSpec{
			CommonRouteSpec: gwapiv1.CommonRouteSpec{
				ParentRefs: []gwapiv1.ParentReference{
					{
						Name: "my-gateway",
					},
				},
			},
			Rules: []gwapiv1.HTTPRouteRule{
				{
					BackendRefs: []gwapiv1.HTTPBackendRef{BuildHTTPBackendRef()},
				},
			},
		},
	}
	for _, fn := range f {
		fn(r)
	}
	return r
}

func BuildHTTPBackendRef(f ...func(*gwapiv1.BackendObjectReference)) gwapiv1.HTTPBackendRef {
	bor := &gwapiv1.BackendObjectReference{
		Name: "my-service",
	}
	for _, fn := range f {
		fn(bor)
	}
	return gwapiv1.HTTPBackendRef{
		BackendRef: gwapiv1.BackendRef{
			BackendObjectReference: *bor,
		},
	}
}

func BuildService(f ...func(*core.Service)) *core.Service {
	s := &core.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: core.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "my-namespace",
		},
		Spec: core.ServiceSpec{
			Ports: []core.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
			Selector: map[string]string{
				"app": "my-app",
			},
		},
	}
	for _, fn := range f {
		fn(s)
	}
	return s
}

type GatewayAPIResources struct {
	GatewayClasses []*gwapiv1.GatewayClass
	Gateways       []*gwapiv1.Gateway
	HTTPRoutes     []*gwapiv1.HTTPRoute
	Services       []*core.Service
}

// BuildComplexGatewayAPITopology returns a set of Gateway API resources organized :
//
//	                                            ┌────────────────┐                                                                        ┌────────────────┐
//	                                            │ gatewayclass-1 │                                                                        │ gatewayclass-2 │
//	                                            └────────────────┘                                                                        └────────────────┘
//	                                                    ▲                                                                                         ▲
//	                                                    │                                                                                         │
//	                          ┌─────────────────────────┼──────────────────────────┐                                                 ┌────────────┴─────────────┐
//	                          │                         │                          │                                                 │                          │
//	          ┌───────────────┴───────────────┐ ┌───────┴────────┐ ┌───────────────┴───────────────┐                  ┌──────────────┴────────────────┐ ┌───────┴────────┐
//	          │           gateway-1           │ │   gateway-2    │ │           gateway-3           │                  │           gateway-4           │ │   gateway-5    │
//	          │                               │ │                │ │                               │                  │                               │ │                │
//	          │ ┌────────────┐ ┌────────────┐ │ │ ┌────────────┐ │ │ ┌────────────┐ ┌────────────┐ │                  │ ┌────────────┐ ┌────────────┐ │ │ ┌────────────┐ │
//	          │ │ listener-1 │ │ listener-2 │ │ │ │ listener-1 │ │ │ │ listener-1 │ │ listener-2 │ │                  │ │ listener-1 │ │ listener-2 │ │ │ │ listener-1 │ │
//	          │ └────────────┘ └────────────┘ │ │ └────────────┘ │ │ └────────────┘ └────────────┘ │                  │ └────────────┘ └────────────┘ │ │ └────────────┘ │
//	          │                        ▲      │ │      ▲         │ │                               │                  │                               │ │                │
//	          └────────────────────────┬──────┘ └──────┬─────────┘ └───────────────────────────────┘                  └───────────────────────────────┘ └────────────────┘
//	                      ▲            │               │     ▲                    ▲            ▲                          ▲           ▲                          ▲
//	                      │            │               │     │                    │            │                          │           │                          │
//	                      │            └───────┬───────┘     │                    │            └────────────┬─────────────┘           │                          │
//	                      │                    │             │                    │                         │                         │                          │
//	          ┌───────────┴───────────┐ ┌──────┴─────┐ ┌─────┴──────┐ ┌───────────┴───────────┐ ┌───────────┴───────────┐ ┌───────────┴───────────┐        ┌─────┴──────┐
//	          │        route-1        │ │  route-2   │ │  route-3   │ │        route-4        │ │        route-5        │ │        route-6        │        │   route-7  │
//	          │                       │ │            │ │            │ │                       │ │                       │ │                       │        │            │
//	          │ ┌────────┐ ┌────────┐ │ │ ┌────────┐ │ │ ┌────────┐ │ │ ┌────────┐ ┌────────┐ │ │ ┌────────┐ ┌────────┐ │ │ ┌────────┐ ┌────────┐ │        │ ┌────────┐ │
//	          │ │ rule-1 │ │ rule-2 │ │ │ │ rule-1 │ │ │ │ rule-1 │ │ │ │ rule-1 │ │ rule-2 │ │ │ │ rule-1 │ │ rule-2 │ │ │ │ rule-1 │ │ rule-2 │ │        │ │ rule-1 │ │
//	          │ └────┬───┘ └────┬───┘ │ │ └────┬───┘ │ │ └───┬────┘ │ │ └─┬──────┘ └───┬────┘ │ │ └───┬────┘ └────┬───┘ │ │ └─┬────┬─┘ └────┬───┘ │        │ └────┬───┘ │
//	          │      │          │     │ │      │     │ │     │      │ │   │            │      │ │     │           │     │ │   │    │        │     │        │      │     │
//	          └──────┼──────────┼─────┘ └──────┼─────┘ └─────┼──────┘ └───┼────────────┼──────┘ └─────┼───────────┼─────┘ └───┼────┼────────┼─────┘        └──────┼─────┘
//	                 │          │              │             │            │            │              │           │           │    │        │                     │
//	                 │          │              └─────────────┤            │            │              └───────────┴───────────┘    │        │                     │
//	                 ▼          ▼                            │            │            │                          ▼                ▼        │                     ▼
//	┌───────────────────────┐ ┌────────────┐          ┌──────┴────────────┴───┐  ┌─────┴──────┐             ┌────────────┐        ┌─────────┴──┐           ┌────────────┐
//	│                       │ │            │          │      ▼            ▼   │  │     ▼      │             │            │        │         ▼  │           │            │
//	│ ┌────────┐ ┌────────┐ │ │ ┌────────┐ │          │ ┌────────┐ ┌────────┐ │  │ ┌────────┐ │             │ ┌────────┐ │        │ ┌────────┐ │           │ ┌────────┐ │
//	│ │ port-1 │ │ port-2 │ │ │ │ port-1 │ │          │ │ port-1 │ │ port-2 │ │  │ │ port-1 │ │             │ │ port-1 │ │        │ │ port-1 │ │           │ │ port-1 │ │
//	│ └────────┘ └────────┘ │ │ └────────┘ │          │ └────────┘ └────────┘ │  │ └────────┘ │             │ └────────┘ │        │ └────────┘ │           │ └────────┘ │
//	│                       │ │            │          │                       │  │            │             │            │        │            │           │            │
//	│       service-1       │ │  service-2 │          │       service-3       │  │  service-4 │             │  service-5 │        │  service-6 │           │  service-7 │
//	└───────────────────────┘ └────────────┘          └───────────────────────┘  └────────────┘             └────────────┘        └────────────┘           └────────────┘
func BuildComplexGatewayAPITopology(funcs ...func(*GatewayAPIResources)) GatewayAPIResources {
	t := GatewayAPIResources{
		GatewayClasses: []*gwapiv1.GatewayClass{
			BuildGatewayClass(func(gc *gwapiv1.GatewayClass) { gc.Name = "gatewayclass-1" }),
			BuildGatewayClass(func(gc *gwapiv1.GatewayClass) { gc.Name = "gatewayclass-2" }),
		},
		Gateways: []*gwapiv1.Gateway{
			BuildGateway(func(g *gwapiv1.Gateway) {
				g.Name = "gateway-1"
				g.Spec.GatewayClassName = "gatewayclass-1"
				g.Spec.Listeners[0].Name = "listener-1"
				g.Spec.Listeners = append(g.Spec.Listeners, gwapiv1.Listener{
					Name:     "listener-2",
					Port:     443,
					Protocol: "HTTPS",
				})
			}),
			BuildGateway(func(g *gwapiv1.Gateway) {
				g.Name = "gateway-2"
				g.Spec.GatewayClassName = "gatewayclass-1"
				g.Spec.Listeners[0].Name = "listener-1"
			}),
			BuildGateway(func(g *gwapiv1.Gateway) {
				g.Name = "gateway-3"
				g.Spec.GatewayClassName = "gatewayclass-1"
				g.Spec.Listeners[0].Name = "listener-1"
				g.Spec.Listeners = append(g.Spec.Listeners, gwapiv1.Listener{
					Name:     "listener-2",
					Port:     443,
					Protocol: "HTTPS",
				})
			}),
			BuildGateway(func(g *gwapiv1.Gateway) {
				g.Name = "gateway-4"
				g.Spec.GatewayClassName = "gatewayclass-2"
				g.Spec.Listeners[0].Name = "listener-1"
				g.Spec.Listeners = append(g.Spec.Listeners, gwapiv1.Listener{
					Name:     "listener-2",
					Port:     443,
					Protocol: "HTTPS",
				})
			}),
			BuildGateway(func(g *gwapiv1.Gateway) {
				g.Name = "gateway-5"
				g.Spec.GatewayClassName = "gatewayclass-2"
				g.Spec.Listeners[0].Name = "listener-1"
			}),
		},
		HTTPRoutes: []*gwapiv1.HTTPRoute{
			BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-1"
				r.Spec.ParentRefs[0].Name = "gateway-1"
				r.Spec.Rules = []gwapiv1.HTTPRouteRule{
					{ // rule-1
						BackendRefs: []gwapiv1.HTTPBackendRef{BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-1"
						})},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.HTTPBackendRef{BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-2"
						})},
					},
				}
			}),
			BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-2"
				r.Spec.ParentRefs = []gwapiv1.ParentReference{
					{
						Name:        "gateway-1",
						SectionName: ptr.To(gwapiv1.SectionName("listener-2")),
					},
					{
						Name:        "gateway-2",
						SectionName: ptr.To(gwapiv1.SectionName("listener-1")),
					},
				}
				r.Spec.Rules[0].BackendRefs[0] = BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
					backendRef.Name = "service-3"
					backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
				})
			}),
			BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-3"
				r.Spec.ParentRefs[0].Name = "gateway-2"
				r.Spec.Rules[0].BackendRefs[0] = BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
					backendRef.Name = "service-3"
					backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
				})
			}),
			BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-4"
				r.Spec.ParentRefs[0].Name = "gateway-3"
				r.Spec.Rules = []gwapiv1.HTTPRouteRule{
					{ // rule-1
						BackendRefs: []gwapiv1.HTTPBackendRef{BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-3"
							backendRef.Port = ptr.To(gwapiv1.PortNumber(443)) // port-2
						})},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.HTTPBackendRef{BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-4"
							backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
						})},
					},
				}
			}),
			BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-5"
				r.Spec.ParentRefs[0].Name = "gateway-3"
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, gwapiv1.ParentReference{Name: "gateway-4"})
				r.Spec.Rules = []gwapiv1.HTTPRouteRule{
					{ // rule-1
						BackendRefs: []gwapiv1.HTTPBackendRef{BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-5"
						})},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.HTTPBackendRef{BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-5"
						})},
					},
				}
			}),
			BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-6"
				r.Spec.ParentRefs[0].Name = "gateway-4"
				r.Spec.Rules = []gwapiv1.HTTPRouteRule{
					{ // rule-1
						BackendRefs: []gwapiv1.HTTPBackendRef{
							BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "service-5"
							}),
							BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "service-6"
							}),
						},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.HTTPBackendRef{BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-6"
							backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
						})},
					},
				}
			}),
			BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-7"
				r.Spec.ParentRefs[0].Name = "gateway-5"
				r.Spec.Rules[0].BackendRefs[0] = BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
					backendRef.Name = "service-7"
				})
			}),
		},
		Services: []*core.Service{
			BuildService(func(s *core.Service) {
				s.Name = "service-1"
				s.Spec.Ports[0].Name = "port-1"
				s.Spec.Ports = append(s.Spec.Ports, core.ServicePort{
					Name: "port-2",
					Port: 443,
				})
			}),
			BuildService(func(s *core.Service) {
				s.Name = "service-2"
				s.Spec.Ports[0].Name = "port-1"
			}),
			BuildService(func(s *core.Service) {
				s.Name = "service-3"
				s.Spec.Ports[0].Name = "port-1"
				s.Spec.Ports = append(s.Spec.Ports, core.ServicePort{
					Name: "port-2",
					Port: 443,
				})
			}),
			BuildService(func(s *core.Service) {
				s.Name = "service-4"
				s.Spec.Ports[0].Name = "port-1"
			}),
			BuildService(func(s *core.Service) {
				s.Name = "service-5"
				s.Spec.Ports[0].Name = "port-1"
			}),
			BuildService(func(s *core.Service) {
				s.Name = "service-6"
				s.Spec.Ports[0].Name = "port-1"
			}),
			BuildService(func(s *core.Service) {
				s.Name = "service-7"
				s.Spec.Ports[0].Name = "port-1"
			}),
		},
	}
	for _, f := range funcs {
		f(&t)
	}
	return t
}

func ListenersFromGatewayFunc(gateway *Gateway, _ int) []*Listener {
	return lo.Map(gateway.Spec.Listeners, func(listener gwapiv1.Listener, _ int) *Listener {
		return &Listener{
			Listener: &listener,
			Gateway:  gateway,
		}
	})
}

func HTTPRouteRulesFromHTTPRouteFunc(httpRoute *HTTPRoute, _ int) []*HTTPRouteRule {
	return lo.Map(httpRoute.Spec.Rules, func(rule gwapiv1.HTTPRouteRule, i int) *HTTPRouteRule {
		return &HTTPRouteRule{
			HTTPRouteRule: &rule,
			HTTPRoute:     httpRoute,
			Name:          gwapiv1.SectionName(fmt.Sprintf("rule-%d", i+1)),
		}
	})
}

func ServicePortsFromBackendFunc(service *Service, _ int) []*ServicePort {
	return lo.Map(service.Spec.Ports, func(port core.ServicePort, _ int) *ServicePort {
		return &ServicePort{
			ServicePort: &port,
			Service:     service,
		}
	})
}

func LinkGatewayClassToGatewayFunc(gatewayClasses []*GatewayClass) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "GatewayClass"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		Func: func(child Targetable) []Targetable {
			gateway := child.(*Gateway)
			gatewayClass, ok := lo.Find(gatewayClasses, func(gc *GatewayClass) bool {
				return gc.Name == string(gateway.Spec.GatewayClassName)
			})
			if ok {
				return []Targetable{gatewayClass}
			}
			return nil
		},
	}
}

func LinkGatewayToHTTPRouteFunc(gateways []*Gateway) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		Func: func(child Targetable) []Targetable {
			httpRoute := child.(*HTTPRoute)
			return lo.FilterMap(httpRoute.Spec.ParentRefs, func(parentRef gwapiv1.ParentReference, _ int) (Targetable, bool) {
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

func LinkGatewayToListenerFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Listener"},
		Func: func(child Targetable) []Targetable {
			listener := child.(*Listener)
			return []Targetable{listener.Gateway}
		},
	}
}

func LinkListenerToHTTPRouteFunc(gateways []*Gateway, listeners []*Listener) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Listener"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		Func: func(child Targetable) []Targetable {
			httpRoute := child.(*HTTPRoute)
			return lo.FlatMap(httpRoute.Spec.ParentRefs, func(parentRef gwapiv1.ParentReference, _ int) []Targetable {
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
					return []Targetable{listener}
				}
				return lo.FilterMap(listeners, func(l *Listener, _ int) (Targetable, bool) {
					return l, l.Gateway.GetURL() == gateway.GetURL()
				})
			})
		},
	}
}

func LinkHTTPRouteToServiceFunc(httpRoutes []*HTTPRoute) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		To:   schema.GroupKind{Kind: "Service"},
		Func: func(child Targetable) []Targetable {
			service := child.(*Service)
			return lo.FilterMap(httpRoutes, func(httpRoute *HTTPRoute, _ int) (Targetable, bool) {
				return httpRoute, lo.ContainsBy(httpRoute.Spec.Rules, func(rule gwapiv1.HTTPRouteRule) bool {
					backendRefs := lo.Map(rule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) gwapiv1.BackendRef { return backendRef.BackendRef })
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, httpRoute.Namespace))
				})
			})
		},
	}
}

func LinkHTTPRouteToHTTPRouteRuleFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		Func: func(child Targetable) []Targetable {
			httpRouteRule := child.(*HTTPRouteRule)
			return []Targetable{httpRouteRule.HTTPRoute}
		},
	}
}

func LinkHTTPRouteRuleToServiceFunc(httpRouteRules []*HTTPRouteRule) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		To:   schema.GroupKind{Kind: "Service"},
		Func: func(child Targetable) []Targetable {
			service := child.(*Service)
			return lo.FilterMap(httpRouteRules, func(httpRouteRule *HTTPRouteRule, _ int) (Targetable, bool) {
				backendRefs := lo.FilterMap(httpRouteRule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) (gwapiv1.BackendRef, bool) {
					return backendRef.BackendRef, backendRef.Port == nil
				})
				return httpRouteRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, httpRouteRule.HTTPRoute.Namespace))
			})
		},
	}
}

func LinkHTTPRouteRuleToServicePortFunc(httpRouteRules []*HTTPRouteRule) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		To:   schema.GroupKind{Kind: "ServicePort"},
		Func: func(child Targetable) []Targetable {
			servicePort := child.(*ServicePort)
			return lo.FilterMap(httpRouteRules, func(httpRouteRule *HTTPRouteRule, _ int) (Targetable, bool) {
				backendRefs := lo.FilterMap(httpRouteRule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) (gwapiv1.BackendRef, bool) {
					return backendRef.BackendRef, backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
				})
				return httpRouteRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(servicePort.Service, httpRouteRule.HTTPRoute.Namespace))
			})
		},
	}
}

func LinkServiceToServicePortFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Kind: "Service"},
		To:   schema.GroupKind{Kind: "ServicePort"},
		Func: func(child Targetable) []Targetable {
			servicePort := child.(*ServicePort)
			return []Targetable{servicePort.Service}
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
