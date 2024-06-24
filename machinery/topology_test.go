//go:build unit

package machinery

import (
	"fmt"
	"testing"

	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type TestPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TestPolicySpec `json:"spec"`
}

var _ Policy = &TestPolicy{}

func (p *TestPolicy) GetURL() string {
	return UrlFromObject(p)
}

func (p *TestPolicy) GetTargetRefs() []PolicyTargetReference {
	return []PolicyTargetReference{LocalPolicyTargetReference{LocalPolicyTargetReference: p.Spec.TargetRef, PolicyNamespace: p.Namespace}}
}

func (p *TestPolicy) GetMergeStrategy() MergeStrategy {
	return DefaultMergeStrategy
}

func (p *TestPolicy) Merge(policy Policy) Policy {
	return &TestPolicy{
		Spec: p.Spec,
	}
}

type TestPolicySpec struct {
	TargetRef gwapiv1alpha2.LocalPolicyTargetReference `json:"targetRef"`
}

func TestTopology(t *testing.T) {
	testCases := []struct {
		name           string
		gatewayClasses []*gwapiv1.GatewayClass
		gateways       []*gwapiv1.Gateway
		httpRoutes     []*gwapiv1.HTTPRoute
		backends       []*core.Service
		policies       []Policy
	}{
		{
			name: "empty",
		},
		{
			name:     "single node",
			gateways: []*gwapiv1.Gateway{buildGateway()},
		},
		{
			name:           "one of each kind",
			gatewayClasses: []*gwapiv1.GatewayClass{buildGatewayClass()},
			gateways:       []*gwapiv1.Gateway{buildGateway()},
			httpRoutes:     []*gwapiv1.HTTPRoute{buildHTTPRoute()},
			backends:       []*core.Service{buildBackend()},
			policies:       []Policy{buildPolicy()},
		},
		{
			name: "complex network",
			//                                             ┌────────────────┐                                                                        ┌────────────────┐
			//                                             │ gatewayclass-1 │                                                                        │ gatewayclass-2 │
			//                                             └────────────────┘                                                                        └────────────────┘
			//                                                     ▲                                                                                         ▲
			//                                                     │                                                                                         │
			//                           ┌─────────────────────────┼──────────────────────────┐                                                 ┌────────────┴─────────────┐
			//                           │                         │                          │                                                 │                          │
			//           ┌───────────────┴───────────────┐ ┌───────┴────────┐ ┌───────────────┴───────────────┐                  ┌──────────────┴────────────────┐ ┌───────┴────────┐
			//           │           gateway-1           │ │   gateway-2    │ │           gateway-3           │                  │           gateway-4           │ │   gateway-5    │
			//           │                               │ │                │ │                               │                  │                               │ │                │
			//           │ ┌────────────┐ ┌────────────┐ │ │ ┌────────────┐ │ │ ┌────────────┐ ┌────────────┐ │                  │ ┌────────────┐ ┌────────────┐ │ │ ┌────────────┐ │
			//           │ │ listener-1 │ │ listener-2 │ │ │ │ listener-1 │ │ │ │ listener-1 │ │ listener-2 │ │                  │ │ listener-1 │ │ listener-2 │ │ │ │ listener-1 │ │
			//           │ └────────────┘ └────────────┘ │ │ └────────────┘ │ │ └────────────┘ └────────────┘ │                  │ └────────────┘ └────────────┘ │ │ └────────────┘ │
			//           │                        ▲      │ │      ▲         │ │                               │                  │                               │ │                │
			//           └────────────────────────┬──────┘ └──────┬─────────┘ └───────────────────────────────┘                  └───────────────────────────────┘ └────────────────┘
			//                       ▲            │               │     ▲                    ▲            ▲                          ▲           ▲                          ▲
			//                       │            │               │     │                    │            │                          │           │                          │
			//                       │            └───────┬───────┘     │                    │            └────────────┬─────────────┘           │                          │
			//                       │                    │             │                    │                         │                         │                          │
			//           ┌───────────┴───────────┐ ┌──────┴─────┐ ┌─────┴──────┐ ┌───────────┴───────────┐ ┌───────────┴───────────┐ ┌───────────┴───────────┐        ┌─────┴──────┐
			//           │        route-1        │ │  route-2   │ │  route-3   │ │        route-4        │ │        route-5        │ │        route-6        │        │   route-7  │
			//           │                       │ │            │ │            │ │                       │ │                       │ │                       │        │            │
			//           │ ┌────────┐ ┌────────┐ │ │ ┌────────┐ │ │ ┌────────┐ │ │ ┌────────┐ ┌────────┐ │ │ ┌────────┐ ┌────────┐ │ │ ┌────────┐ ┌────────┐ │        │ ┌────────┐ │
			//           │ │ rule-1 │ │ rule-2 │ │ │ │ rule-1 │ │ │ │ rule-1 │ │ │ │ rule-1 │ │ rule-2 │ │ │ │ rule-1 │ │ rule-2 │ │ │ │ rule-1 │ │ rule-2 │ │        │ │ rule-1 │ │
			//           │ └────┬───┘ └────┬───┘ │ │ └────┬───┘ │ │ └───┬────┘ │ │ └─┬──────┘ └───┬────┘ │ │ └───┬────┘ └────┬───┘ │ │ └─┬────┬─┘ └────┬───┘ │        │ └────┬───┘ │
			//           │      │          │     │ │      │     │ │     │      │ │   │            │      │ │     │           │     │ │   │    │        │     │        │      │     │
			//           └──────┼──────────┼─────┘ └──────┼─────┘ └─────┼──────┘ └───┼────────────┼──────┘ └─────┼───────────┼─────┘ └───┼────┼────────┼─────┘        └──────┼─────┘
			//                  │          │              │             │            │            │              │           │           │    │        │                     │
			//                  │          │              └─────────────┤            │            │              └───────────┴───────────┘    │        │                     │
			//                  ▼          ▼                            │            │            │                          ▼                ▼        │                     ▼
			// ┌───────────────────────┐ ┌────────────┐          ┌──────┴────────────┴───┐  ┌─────┴──────┐             ┌────────────┐        ┌─────────┴──┐           ┌────────────┐
			// │                       │ │            │          │      ▼            ▼   │  │     ▼      │             │            │        │         ▼  │           │            │
			// │ ┌────────┐ ┌────────┐ │ │ ┌────────┐ │          │ ┌────────┐ ┌────────┐ │  │ ┌────────┐ │             │ ┌────────┐ │        │ ┌────────┐ │           │ ┌────────┐ │
			// │ │ port-1 │ │ port-2 │ │ │ │ port-1 │ │          │ │ port-1 │ │ port-2 │ │  │ │ port-1 │ │             │ │ port-1 │ │        │ │ port-1 │ │           │ │ port-1 │ │
			// │ └────────┘ └────────┘ │ │ └────────┘ │          │ └────────┘ └────────┘ │  │ └────────┘ │             │ └────────┘ │        │ └────────┘ │           │ └────────┘ │
			// │                       │ │            │          │                       │  │            │             │            │        │            │           │            │
			// │       backend-1       │ │  backend-2 │          │       backend-3       │  │  backend-4 │             │  backend-5 │        │  backend-6 │           │  backend-7 │
			// └───────────────────────┘ └────────────┘          └───────────────────────┘  └────────────┘             └────────────┘        └────────────┘           └────────────┘
			gatewayClasses: []*gwapiv1.GatewayClass{
				buildGatewayClass(func(gc *gwapiv1.GatewayClass) { gc.Name = "gatewayclass-1" }),
				buildGatewayClass(func(gc *gwapiv1.GatewayClass) { gc.Name = "gatewayclass-2" }),
			},
			gateways: []*gwapiv1.Gateway{
				buildGateway(func(g *gwapiv1.Gateway) {
					g.Name = "gateway-1"
					g.Spec.GatewayClassName = "gatewayclass-1"
					g.Spec.Listeners[0].Name = "listener-1"
					g.Spec.Listeners = append(g.Spec.Listeners, gwapiv1.Listener{
						Name:     "listener-2",
						Port:     443,
						Protocol: "HTTPS",
					})
				}),
				buildGateway(func(g *gwapiv1.Gateway) {
					g.Name = "gateway-2"
					g.Spec.GatewayClassName = "gatewayclass-1"
					g.Spec.Listeners[0].Name = "listener-1"
				}),
				buildGateway(func(g *gwapiv1.Gateway) {
					g.Name = "gateway-3"
					g.Spec.GatewayClassName = "gatewayclass-1"
					g.Spec.Listeners[0].Name = "listener-1"
					g.Spec.Listeners = append(g.Spec.Listeners, gwapiv1.Listener{
						Name:     "listener-2",
						Port:     443,
						Protocol: "HTTPS",
					})
				}),
				buildGateway(func(g *gwapiv1.Gateway) {
					g.Name = "gateway-4"
					g.Spec.GatewayClassName = "gatewayclass-2"
					g.Spec.Listeners[0].Name = "listener-1"
					g.Spec.Listeners = append(g.Spec.Listeners, gwapiv1.Listener{
						Name:     "listener-2",
						Port:     443,
						Protocol: "HTTPS",
					})
				}),
				buildGateway(func(g *gwapiv1.Gateway) {
					g.Name = "gateway-5"
					g.Spec.GatewayClassName = "gatewayclass-2"
					g.Spec.Listeners[0].Name = "listener-1"
				}),
			},
			httpRoutes: []*gwapiv1.HTTPRoute{
				buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
					r.Name = "route-1"
					r.Spec.ParentRefs[0].Name = "gateway-1"
					r.Spec.Rules = []gwapiv1.HTTPRouteRule{
						{ // rule-1
							BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "backend-1"
							})},
						},
						{ // rule-2
							BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "backend-2"
							})},
						},
					}
				}),
				buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
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
					r.Spec.Rules[0].BackendRefs[0] = buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
						backendRef.Name = "backend-3"
						backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
					})
				}),
				buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
					r.Name = "route-3"
					r.Spec.ParentRefs[0].Name = "gateway-2"
					r.Spec.Rules[0].BackendRefs[0] = buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
						backendRef.Name = "backend-3"
						backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
					})
				}),
				buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
					r.Name = "route-4"
					r.Spec.ParentRefs[0].Name = "gateway-3"
					r.Spec.Rules = []gwapiv1.HTTPRouteRule{
						{ // rule-1
							BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "backend-3"
								backendRef.Port = ptr.To(gwapiv1.PortNumber(443)) // port-2
							})},
						},
						{ // rule-2
							BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "backend-4"
							})},
						},
					}
				}),
				buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
					r.Name = "route-5"
					r.Spec.ParentRefs[0].Name = "gateway-3"
					r.Spec.ParentRefs = append(r.Spec.ParentRefs, gwapiv1.ParentReference{Name: "gateway-4"})
					r.Spec.Rules = []gwapiv1.HTTPRouteRule{
						{ // rule-1
							BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "backend-5"
							})},
						},
						{ // rule-2
							BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "backend-5"
							})},
						},
					}
				}),
				buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
					r.Name = "route-6"
					r.Spec.ParentRefs[0].Name = "gateway-4"
					r.Spec.Rules = []gwapiv1.HTTPRouteRule{
						{ // rule-1
							BackendRefs: []gwapiv1.HTTPBackendRef{
								buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
									backendRef.Name = "backend-5"
								}),
								buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
									backendRef.Name = "backend-6"
								}),
							},
						},
						{ // rule-2
							BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "backend-6"
								backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
							})},
						},
					}
				}),
				buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
					r.Name = "route-7"
					r.Spec.ParentRefs[0].Name = "gateway-5"
					r.Spec.Rules[0].BackendRefs[0] = buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
						backendRef.Name = "backend-7"
					})
				}),
			},
			backends: []*core.Service{
				buildBackend(func(s *core.Service) {
					s.Name = "backend-1"
					s.Spec.Ports[0].Name = "port-1"
					s.Spec.Ports = append(s.Spec.Ports, core.ServicePort{
						Name: "port-2",
						Port: 443,
					})
				}),
				buildBackend(func(s *core.Service) {
					s.Name = "backend-2"
					s.Spec.Ports[0].Name = "port-1"
				}),
				buildBackend(func(s *core.Service) {
					s.Name = "backend-3"
					s.Spec.Ports[0].Name = "port-1"
					s.Spec.Ports = append(s.Spec.Ports, core.ServicePort{
						Name: "port-2",
						Port: 443,
					})
				}),
				buildBackend(func(s *core.Service) {
					s.Name = "backend-4"
					s.Spec.Ports[0].Name = "port-1"
				}),
				buildBackend(func(s *core.Service) {
					s.Name = "backend-5"
					s.Spec.Ports[0].Name = "port-1"
				}),
				buildBackend(func(s *core.Service) {
					s.Name = "backend-6"
					s.Spec.Ports[0].Name = "port-1"
				}),
				buildBackend(func(s *core.Service) {
					s.Name = "backend-7"
					s.Spec.Ports[0].Name = "port-1"
				}),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			targetables := make([]Targetable, 0, len(tc.gatewayClasses)+len(tc.gateways)+len(tc.httpRoutes)+len(tc.backends))
			targetables = append(targetables, lo.Map(tc.gatewayClasses, func(gatewayClass *gwapiv1.GatewayClass, _ int) Targetable {
				return GatewayClass{GatewayClass: gatewayClass}
			})...)
			targetables = append(targetables, lo.Map(tc.gateways, func(gateway *gwapiv1.Gateway, _ int) Targetable { return Gateway{Gateway: gateway} })...)
			targetables = append(targetables, lo.Map(tc.httpRoutes, func(httpRoute *gwapiv1.HTTPRoute, _ int) Targetable { return HTTPRoute{HTTPRoute: httpRoute} })...)
			targetables = append(targetables, lo.Map(tc.backends, func(service *core.Service, _ int) Targetable { return Backend{Service: service} })...)
			topology := NewTopology(targetables, tc.policies)
			fmt.Println(topology.ToDot())
		})
	}
}

func buildGatewayClass(f ...func(*gwapiv1.GatewayClass)) *gwapiv1.GatewayClass {
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

func buildGateway(f ...func(*gwapiv1.Gateway)) *gwapiv1.Gateway {
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

func buildHTTPRoute(f ...func(*gwapiv1.HTTPRoute)) *gwapiv1.HTTPRoute {
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
					BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef()},
				},
			},
		},
	}
	for _, fn := range f {
		fn(r)
	}
	return r
}

func buildHTTPBackendRef(f ...func(*gwapiv1.BackendObjectReference)) gwapiv1.HTTPBackendRef {
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

func buildBackend(f ...func(*core.Service)) *core.Service {
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
			TargetRef: gwapiv1alpha2.LocalPolicyTargetReference{
				Kind: "Service",
				Name: "my-service",
			},
		},
	}
	for _, fn := range f {
		fn(p)
	}
	return p
}
