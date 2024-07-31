//go:build unit || integration

package machinery

import (
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
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
	return gwapiv1.HTTPBackendRef{
		BackendRef: BuildBackendRef(f...),
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

func BuildGRPCRoute(f ...func(*gwapiv1.GRPCRoute)) *gwapiv1.GRPCRoute {
	r := &gwapiv1.GRPCRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gwapiv1.GroupVersion.String(),
			Kind:       "GRPCRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-grpc-route",
			Namespace: "my-namespace",
		},
		Spec: gwapiv1.GRPCRouteSpec{
			CommonRouteSpec: gwapiv1.CommonRouteSpec{
				ParentRefs: []gwapiv1.ParentReference{
					{
						Name: "my-gateway",
					},
				},
			},
			Rules: []gwapiv1.GRPCRouteRule{
				{
					BackendRefs: []gwapiv1.GRPCBackendRef{BuildGRPCBackendRef()},
				},
			},
		},
	}
	for _, fn := range f {
		fn(r)
	}

	return r
}

func BuildGRPCBackendRef(f ...func(*gwapiv1.BackendObjectReference)) gwapiv1.GRPCBackendRef {
	return gwapiv1.GRPCBackendRef{
		BackendRef: BuildBackendRef(f...),
	}
}

func BuildBackendRef(f ...func(*gwapiv1.BackendObjectReference)) gwapiv1.BackendRef {
	bor := &gwapiv1.BackendObjectReference{
		Name: "my-service",
	}
	for _, fn := range f {
		fn(bor)
	}
	return gwapiv1.BackendRef{
		BackendObjectReference: *bor,
	}
}

func BuildTCPRoute(f ...func(route *gwapiv1alpha2.TCPRoute)) *gwapiv1alpha2.TCPRoute {
	r := &gwapiv1alpha2.TCPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gwapiv1.GroupVersion.String(),
			Kind:       "TCPRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-tcp-route",
			Namespace: "my-namespace",
		},
		Spec: gwapiv1alpha2.TCPRouteSpec{
			CommonRouteSpec: gwapiv1.CommonRouteSpec{
				ParentRefs: []gwapiv1.ParentReference{
					{
						Name: "my-gateway",
					},
				},
			},
			Rules: []gwapiv1alpha2.TCPRouteRule{
				{
					BackendRefs: []gwapiv1.BackendRef{BuildBackendRef()},
				},
			},
		},
	}
	for _, fn := range f {
		fn(r)
	}

	return r
}

func BuildTLSRoute(f ...func(route *gwapiv1alpha2.TLSRoute)) *gwapiv1alpha2.TLSRoute {
	r := &gwapiv1alpha2.TLSRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gwapiv1.GroupVersion.String(),
			Kind:       "TLSRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-tls-route",
			Namespace: "my-namespace",
		},
		Spec: gwapiv1alpha2.TLSRouteSpec{
			CommonRouteSpec: gwapiv1.CommonRouteSpec{
				ParentRefs: []gwapiv1.ParentReference{
					{
						Name: "my-gateway",
					},
				},
			},
			Rules: []gwapiv1alpha2.TLSRouteRule{
				{
					BackendRefs: []gwapiv1.BackendRef{BuildBackendRef()},
				},
			},
		},
	}
	for _, fn := range f {
		fn(r)
	}

	return r
}

type GatewayAPIResources struct {
	GatewayClasses []*gwapiv1.GatewayClass
	Gateways       []*gwapiv1.Gateway
	HTTPRoutes     []*gwapiv1.HTTPRoute
	GRPCRoutes     []*gwapiv1.GRPCRoute
	TCPRoutes      []*gwapiv1alpha2.TCPRoute
	TLSRoutes      []*gwapiv1alpha2.TLSRoute
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
//	          │        h-route-1      │ │ h-route-2  │ │ h-route-3  │ │        h-route-4      │ │        h-route-5      │ │        tcp-route-1    │        │  g-route-1 │
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
				r.Name = "http-route-1"
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
				r.Name = "http-route-2"
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
				r.Name = "http-route-3"
				r.Spec.ParentRefs[0].Name = "gateway-2"
				r.Spec.Rules[0].BackendRefs[0] = BuildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
					backendRef.Name = "service-3"
					backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
				})
			}),
			BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "http-route-4"
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
		GRPCRoutes: []*gwapiv1.GRPCRoute{
			BuildGRPCRoute(func(r *gwapiv1.GRPCRoute) {
				r.Name = "grpc-route-1"
				r.Spec.ParentRefs[0].Name = "gateway-5"
				r.Spec.Rules[0].BackendRefs[0] = BuildGRPCBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
					backendRef.Name = "service-7"
				})
			}),
		},
		TCPRoutes: []*gwapiv1alpha2.TCPRoute{
			BuildTCPRoute(func(r *gwapiv1alpha2.TCPRoute) {
				r.Name = "tcp-route-1"
				r.Spec.ParentRefs[0].Name = "gateway-4"
				r.Spec.Rules = []gwapiv1alpha2.TCPRouteRule{
					{ // rule-1
						BackendRefs: []gwapiv1.BackendRef{
							BuildBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "service-5"
							}),
							BuildBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "service-6"
							}),
						},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.BackendRef{BuildBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-6"
							backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
						})},
					},
				}
			}),
		},
		TLSRoutes: []*gwapiv1alpha2.TLSRoute{
			BuildTLSRoute(func(r *gwapiv1alpha2.TLSRoute) {
				r.Name = "tls-route-1"
				r.Spec.ParentRefs[0].Name = "gateway-3"
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, gwapiv1.ParentReference{Name: "gateway-4"})
				r.Spec.Rules = []gwapiv1alpha2.TLSRouteRule{
					{ // rule-1
						BackendRefs: []gwapiv1.BackendRef{BuildBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-5"
						})},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.BackendRef{BuildBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-5"
						})},
					},
				}
			}),
		},
	}
	for _, f := range funcs {
		f(&t)
	}
	return t
}

type TestPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TestPolicySpec `json:"spec"`
}

type TestPolicySpec struct {
	TargetRef gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`
}

var _ Policy = &TestPolicy{}

func (p *TestPolicy) GetURL() string {
	return UrlFromObject(p)
}

func (p *TestPolicy) GetTargetRefs() []PolicyTargetReference {
	return []PolicyTargetReference{
		LocalPolicyTargetReferenceWithSectionName{
			LocalPolicyTargetReferenceWithSectionName: p.Spec.TargetRef,
			PolicyNamespace: p.Namespace,
		},
	}
}

func (p *TestPolicy) GetMergeStrategy() MergeStrategy {
	return DefaultMergeStrategy
}

func (p *TestPolicy) Merge(policy Policy) Policy {
	return &TestPolicy{
		Spec: p.Spec,
	}
}

func buildPolicy(f ...func(*TestPolicy)) *TestPolicy {
	p := &TestPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test/v1",
			Kind:       "TestPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-policy",
			Namespace: "my-namespace",
		},
		Spec: TestPolicySpec{
			TargetRef: gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
				LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
					Group: gwapiv1.Group(core.SchemeGroupVersion.Group),
					Kind:  "Service",
					Name:  "my-service",
				},
			},
		},
	}
	for _, fn := range f {
		fn(p)
	}
	return p
}
