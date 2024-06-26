//go:build unit

package machinery

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const outDir = "../tests/out"

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
	return []PolicyTargetReference{LocalPolicyTargetReferenceWithSectionName{LocalPolicyTargetReferenceWithSectionName: p.Spec.TargetRef, PolicyNamespace: p.Namespace}}
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
	TargetRef gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`
}

func TestTopologyRoots(t *testing.T) {
	gateways := []Gateway{
		{Gateway: buildGateway()},
		{Gateway: buildGateway(func(g *gwapiv1.Gateway) { g.Name = "my-gateway-2" })},
	}
	httpRoute := HTTPRoute{HTTPRoute: buildHTTPRoute()}
	httpRouteRules := httpRouteRulesFromHTTPRouteFunc(httpRoute, 0)
	topology := NewTopology(
		WithTargetables(gateways...),
		WithTargetables(httpRoute),
		WithTargetables(httpRouteRules...),
		WithLinks(
			linkGatewayToHTTPRouteFunc(gateways),
			linkHTTPRouteToHTTPRouteRuleFunc(),
		),
		WithPolicies(
			buildPolicy(func(policy *TestPolicy) {
				policy.Name = "my-policy-1"
				policy.Spec.TargetRef = gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
					LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
						Group: gwapiv1.GroupName,
						Kind:  "HTTPRoute",
						Name:  "my-http-route",
					},
				}
			}),
			buildPolicy(func(policy *TestPolicy) {
				policy.Name = "my-policy-2"
				policy.Spec.TargetRef = gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
					LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
						Group: gwapiv1.GroupName,
						Kind:  "Gateway",
						Name:  "my-gateway-2",
					},
				}
			}),
		),
	)
	roots := topology.Roots()
	if expected := len(gateways); len(roots) != expected {
		t.Errorf("expected %d roots, got %d", expected, len(roots))
	}
	rootURLs := lo.Map(roots, mapTargetableToURLFunc)
	for _, gateway := range gateways {
		if !lo.Contains(rootURLs, gateway.GetURL()) {
			t.Errorf("expected root %s not found", gateway.GetURL())
		}
	}
}

func TestTopologyParents(t *testing.T) {
	gateways := []Gateway{{Gateway: buildGateway()}}
	httpRoute := HTTPRoute{HTTPRoute: buildHTTPRoute()}
	httpRouteRules := httpRouteRulesFromHTTPRouteFunc(httpRoute, 0)
	topology := NewTopology(
		WithTargetables(gateways...),
		WithTargetables(httpRoute),
		WithTargetables(httpRouteRules...),
		WithLinks(
			linkGatewayToHTTPRouteFunc(gateways),
			linkHTTPRouteToHTTPRouteRuleFunc(),
		),
		WithPolicies(
			buildPolicy(func(policy *TestPolicy) {
				policy.Spec.TargetRef = gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
					LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
						Group: gwapiv1.GroupName,
						Kind:  "HTTPRoute",
						Name:  "my-http-route",
					},
				}
			}),
		),
	)
	parents := topology.Parents(httpRoute)
	if expected := 1; len(parents) != expected {
		t.Errorf("expected %d parent, got %d", expected, len(parents))
	}
	parentURLs := lo.Map(parents, mapTargetableToURLFunc)
	for _, gateway := range gateways {
		if !lo.Contains(parentURLs, gateway.GetURL()) {
			t.Errorf("expected parent %s not found", gateway.GetURL())
		}
	}
}

func TestTopologyChildren(t *testing.T) {
	gateways := []Gateway{{Gateway: buildGateway()}}
	httpRoute := HTTPRoute{HTTPRoute: buildHTTPRoute()}
	httpRouteRules := httpRouteRulesFromHTTPRouteFunc(httpRoute, 0)
	topology := NewTopology(
		WithTargetables(gateways...),
		WithTargetables(httpRoute),
		WithTargetables(httpRouteRules...),
		WithLinks(
			linkGatewayToHTTPRouteFunc(gateways),
			linkHTTPRouteToHTTPRouteRuleFunc(),
		),
		WithPolicies(
			buildPolicy(func(policy *TestPolicy) {
				policy.Spec.TargetRef = gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
					LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
						Group: gwapiv1.GroupName,
						Kind:  "HTTPRoute",
						Name:  "my-http-route",
					},
				}
			}),
		),
	)
	children := topology.Children(httpRoute)
	if expected := 1; len(children) != expected {
		t.Errorf("expected %d child, got %d", expected, len(children))
	}
	childURLs := lo.Map(children, mapTargetableToURLFunc)
	for _, httpRouteRule := range httpRouteRules {
		if !lo.Contains(childURLs, httpRouteRule.GetURL()) {
			t.Errorf("expected child %s not found", httpRouteRule.GetURL())
		}
	}
}

func TestTopologyPaths(t *testing.T) {
	gateways := []Gateway{{Gateway: buildGateway()}}
	httpRoutes := []HTTPRoute{
		{HTTPRoute: buildHTTPRoute(func(r *gwapiv1.HTTPRoute) { r.Name = "route-1" })},
		{HTTPRoute: buildHTTPRoute(func(r *gwapiv1.HTTPRoute) { r.Name = "route-2" })},
	}
	services := []Service{{Service: buildService()}}
	topology := NewTopology(
		WithTargetables(gateways...),
		WithTargetables(httpRoutes...),
		WithTargetables(services...),
		WithLinks(
			linkGatewayToHTTPRouteFunc(gateways),
			linkHTTPRouteToServiceFunc(httpRoutes),
		),
	)

	testCases := []struct {
		name          string
		from          Targetable
		to            Targetable
		expectedPaths [][]Targetable
	}{
		{
			name: "single path",
			from: httpRoutes[0],
			to:   services[0],
			expectedPaths: [][]Targetable{
				{httpRoutes[0], services[0]},
			},
		},
		{
			name: "multiple paths",
			from: gateways[0],
			to:   services[0],
			expectedPaths: [][]Targetable{
				{gateways[0], httpRoutes[0], services[0]},
				{gateways[0], httpRoutes[1], services[0]},
			},
		},
		{
			name: "trivial path",
			from: gateways[0],
			to:   gateways[0],
			expectedPaths: [][]Targetable{
				{gateways[0]},
			},
		},
		{
			name:          "no path",
			from:          services[0],
			to:            gateways[0],
			expectedPaths: [][]Targetable{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths := topology.Paths(tc.from, tc.to)
			if len(paths) != len(tc.expectedPaths) {
				t.Errorf("expected %d paths, got %d", len(tc.expectedPaths), len(paths))
			}
			expectedPaths := lo.Map(tc.expectedPaths, func(expectedPath []Targetable, _ int) string {
				return strings.Join(lo.Map(expectedPath, mapTargetableToURLFunc), "→")
			})
			for _, path := range paths {
				pathString := strings.Join(lo.Map(path, mapTargetableToURLFunc), "→")
				if !lo.Contains(expectedPaths, pathString) {
					t.Errorf("expected path %v not found", pathString)
				}
			}
		})
	}
}

type gwapiTopologyTestCase struct {
	name           string
	gatewayClasses []*gwapiv1.GatewayClass
	gateways       []*gwapiv1.Gateway
	httpRoutes     []*gwapiv1.HTTPRoute
	services       []*core.Service
	policies       []Policy
	expectedLinks  map[string][]string
}

// TestGatewayAPITopology tests for a topology of Gateway API resources with the following architecture:
//
//	GatewayClass -> Gateway -> Listener -> HTTPRoute -> HTTPRouteRule -> Service -> ServicePort
//	                                                                  ∟> ServicePort <- Service
func TestGatewayAPITopology(t *testing.T) {
	testCases := []gwapiTopologyTestCase{
		{
			name: "empty",
		},
		{
			name:           "single node",
			gatewayClasses: []*gwapiv1.GatewayClass{buildGatewayClass()},
		},
		{
			name:           "one of each kind",
			gatewayClasses: []*gwapiv1.GatewayClass{buildGatewayClass()},
			gateways:       []*gwapiv1.Gateway{buildGateway()},
			httpRoutes:     []*gwapiv1.HTTPRoute{buildHTTPRoute()},
			services:       []*core.Service{buildService()},
			policies:       []Policy{buildPolicy()},
			expectedLinks: map[string][]string{
				"my-gateway-class":       {"my-gateway"},
				"my-gateway":             {"my-gateway#my-listener"},
				"my-gateway#my-listener": {"my-http-route"},
				"my-http-route":          {"my-http-route#rule-1"},
				"my-http-route#rule-1":   {"my-service"},
				"my-service":             {"my-service#http"},
			},
		},
		{
			name: "policies with section names",
			gateways: []*gwapiv1.Gateway{buildGateway(func(gateway *gwapiv1.Gateway) {
				gateway.Spec.Listeners[0].Name = "http"
				gateway.Spec.Listeners = append(gateway.Spec.Listeners, gwapiv1.Listener{
					Name:     "https",
					Port:     443,
					Protocol: "HTTPS",
				})
			})},
			policies: []Policy{
				buildPolicy(func(policy *TestPolicy) {
					policy.Name = "my-policy-1"
					policy.Spec.TargetRef = gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
						LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
							Group: gwapiv1.GroupName,
							Kind:  "Gateway",
							Name:  "my-gateway",
						},
						SectionName: ptr.To(gwapiv1.SectionName("http")),
					}
				}),
				buildPolicy(func(policy *TestPolicy) {
					policy.Name = "my-policy-2"
					policy.Spec.TargetRef = gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
						LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
							Group: gwapiv1.GroupName,
							Kind:  "Gateway",
							Name:  "my-gateway",
						},
						SectionName: ptr.To(gwapiv1.SectionName("https")),
					}
				}),
			},
			expectedLinks: map[string][]string{
				"my-gateway": {"my-gateway#http", "my-gateway#https"},
			},
		},
		complexTopologyTestCase(func(tc *gwapiTopologyTestCase) {
			tc.expectedLinks = map[string][]string{
				"gatewayclass-1":       {"gateway-1", "gateway-2", "gateway-3"},
				"gatewayclass-2":       {"gateway-4", "gateway-5"},
				"gateway-1":            {"gateway-1#listener-1", "gateway-1#listener-2"},
				"gateway-2":            {"gateway-2#listener-1"},
				"gateway-3":            {"gateway-3#listener-1", "gateway-3#listener-2"},
				"gateway-4":            {"gateway-4#listener-1", "gateway-4#listener-2"},
				"gateway-5":            {"gateway-5#listener-1"},
				"gateway-1#listener-1": {"route-1"},
				"gateway-1#listener-2": {"route-1", "route-2"},
				"gateway-2#listener-1": {"route-2", "route-3"},
				"gateway-3#listener-1": {"route-4", "route-5"},
				"gateway-3#listener-2": {"route-4", "route-5"},
				"gateway-4#listener-1": {"route-5", "route-6"},
				"gateway-4#listener-2": {"route-5", "route-6"},
				"gateway-5#listener-1": {"route-7"},
				"route-1":              {"route-1#rule-1", "route-1#rule-2"},
				"route-2":              {"route-2#rule-1"},
				"route-3":              {"route-3#rule-1"},
				"route-4":              {"route-4#rule-1", "route-4#rule-2"},
				"route-5":              {"route-5#rule-1", "route-5#rule-2"},
				"route-6":              {"route-6#rule-1", "route-6#rule-2"},
				"route-7":              {"route-7#rule-1"},
				"route-1#rule-1":       {"service-1"},
				"route-1#rule-2":       {"service-2"},
				"route-2#rule-1":       {"service-3#port-1"},
				"route-3#rule-1":       {"service-3#port-1"},
				"route-4#rule-1":       {"service-3#port-2"},
				"route-4#rule-2":       {"service-4#port-1"},
				"route-5#rule-1":       {"service-5"},
				"route-5#rule-2":       {"service-5"},
				"route-6#rule-1":       {"service-5", "service-6"},
				"route-6#rule-2":       {"service-6#port-1"},
				"route-7#rule-1":       {"service-7"},
				"service-1":            {"service-1#port-1", "service-1#port-2"},
				"service-2":            {"service-2#port-1"},
				"service-3":            {"service-3#port-1", "service-3#port-2"},
				"service-4":            {"service-4#port-1"},
				"service-5":            {"service-5#port-1"},
				"service-6":            {"service-6#port-1"},
				"service-7":            {"service-7#port-1"},
			}
		}),
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gatewayClasses := lo.Map(tc.gatewayClasses, func(gatewayClass *gwapiv1.GatewayClass, _ int) GatewayClass {
				return GatewayClass{GatewayClass: gatewayClass}
			})
			gateways := lo.Map(tc.gateways, func(gateway *gwapiv1.Gateway, _ int) Gateway { return Gateway{Gateway: gateway} })
			listeners := lo.FlatMap(gateways, listenersFromGatewayFunc)
			httpRoutes := lo.Map(tc.httpRoutes, func(httpRoute *gwapiv1.HTTPRoute, _ int) HTTPRoute { return HTTPRoute{HTTPRoute: httpRoute} })
			httpRouteRules := lo.FlatMap(httpRoutes, httpRouteRulesFromHTTPRouteFunc)
			services := lo.Map(tc.services, func(service *core.Service, _ int) Service { return Service{Service: service} })
			servicePorts := lo.FlatMap(services, servicePortsFromBackendFunc)

			topology := NewTopology(
				WithTargetables(gatewayClasses...),
				WithTargetables(gateways...),
				WithTargetables(listeners...),
				WithTargetables(httpRoutes...),
				WithTargetables(httpRouteRules...),
				WithTargetables(services...),
				WithTargetables(servicePorts...),
				WithLinks(
					linkGatewayClassToGatewayFunc(gatewayClasses),
					linkGatewayToListenerFunc(),
					linkListenerToHTTPRouteFunc(gateways, listeners),
					linkHTTPRouteToHTTPRouteRuleFunc(),
					linkHTTPRouteRuleToServiceFunc(httpRouteRules),
					linkHTTPRouteRuleToServicePortFunc(httpRouteRules),
					linkServiceToServicePortFunc(),
				),
				WithPolicies(tc.policies...),
			)

			links := make(map[string][]string)
			for _, root := range topology.Roots() {
				linksFromNode(topology, root, links)
			}
			for from, tos := range links {
				expectedTos := tc.expectedLinks[from]
				slices.Sort(expectedTos)
				slices.Sort(tos)
				if !slices.Equal(expectedTos, tos) {
					t.Errorf("expected links from %s to be %v, got %v", from, expectedTos, tos)
				}
			}

			saveTestCaseOutput(t, topology.ToDot())
		})
	}
}

// TestGatewayAPITopologyWithoutSectionName tests for a simplified topology of Gateway API resources without
// section names, i.e. where HTTPRoutes are not expanded to link to specific Listeners, and Policy TargetRefs
// are not of LocalPolicyTargetReferenceWithSectionName kind. This results in the following architecture:
//
//	GatewayClass -> Gateway -> HTTPRoute -> Service
func TestGatewayAPITopologyWithoutSectionName(t *testing.T) {
	testCases := []gwapiTopologyTestCase{
		{
			name:           "one of each kind",
			gatewayClasses: []*gwapiv1.GatewayClass{buildGatewayClass()},
			gateways:       []*gwapiv1.Gateway{buildGateway()},
			httpRoutes:     []*gwapiv1.HTTPRoute{buildHTTPRoute()},
			services:       []*core.Service{buildService()},
			policies:       []Policy{buildPolicy()},
			expectedLinks: map[string][]string{
				"my-gateway-class": {"my-gateway"},
				"my-gateway":       {"my-http-route"},
				"my-http-route":    {"my-service"},
			},
		},
		complexTopologyTestCase(func(tc *gwapiTopologyTestCase) {
			tc.expectedLinks = map[string][]string{
				"gatewayclass-1": {"gateway-1", "gateway-2", "gateway-3"},
				"gatewayclass-2": {"gateway-4", "gateway-5"},
				"gateway-1":      {"route-1", "route-2"},
				"gateway-2":      {"route-2", "route-3"},
				"gateway-3":      {"route-4", "route-5"},
				"gateway-4":      {"route-5", "route-6"},
				"gateway-5":      {"route-7"},
				"route-1":        {"service-1", "service-2"},
				"route-2":        {"service-3"},
				"route-3":        {"service-3"},
				"route-4":        {"service-3", "service-4"},
				"route-5":        {"service-5"},
				"route-6":        {"service-5", "service-6"},
				"route-7":        {"service-7"},
			}
		}),
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gatewayClasses := lo.Map(tc.gatewayClasses, func(gatewayClass *gwapiv1.GatewayClass, _ int) GatewayClass {
				return GatewayClass{GatewayClass: gatewayClass}
			})
			gateways := lo.Map(tc.gateways, func(gateway *gwapiv1.Gateway, _ int) Gateway { return Gateway{Gateway: gateway} })
			httpRoutes := lo.Map(tc.httpRoutes, func(httpRoute *gwapiv1.HTTPRoute, _ int) HTTPRoute { return HTTPRoute{HTTPRoute: httpRoute} })
			services := lo.Map(tc.services, func(service *core.Service, _ int) Service { return Service{Service: service} })

			topology := NewTopology(
				WithTargetables(gatewayClasses...),
				WithTargetables(gateways...),
				WithTargetables(httpRoutes...),
				WithTargetables(services...),
				WithLinks(
					linkGatewayClassToGatewayFunc(gatewayClasses),
					linkGatewayToHTTPRouteFunc(gateways),
					linkHTTPRouteToServiceFunc(httpRoutes),
				),
				WithPolicies(tc.policies...),
			)

			links := make(map[string][]string)
			for _, root := range topology.Roots() {
				linksFromNode(topology, root, links)
			}
			for from, tos := range links {
				expectedTos := tc.expectedLinks[from]
				slices.Sort(expectedTos)
				slices.Sort(tos)
				if !slices.Equal(expectedTos, tos) {
					t.Errorf("expected links from %s to be %v, got %v", from, expectedTos, tos)
				}
			}

			saveTestCaseOutput(t, topology.ToDot())
		})
	}
}

// complexTopologyTestCase returns a gwapiTopologyTestCase for the following complex network of Gateway API resources:
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
func complexTopologyTestCase(opts ...func(*gwapiTopologyTestCase)) gwapiTopologyTestCase {
	tc := gwapiTopologyTestCase{
		name: "complex topology",
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
							backendRef.Name = "service-1"
						})},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-2"
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
					backendRef.Name = "service-3"
					backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
				})
			}),
			buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-3"
				r.Spec.ParentRefs[0].Name = "gateway-2"
				r.Spec.Rules[0].BackendRefs[0] = buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
					backendRef.Name = "service-3"
					backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
				})
			}),
			buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-4"
				r.Spec.ParentRefs[0].Name = "gateway-3"
				r.Spec.Rules = []gwapiv1.HTTPRouteRule{
					{ // rule-1
						BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-3"
							backendRef.Port = ptr.To(gwapiv1.PortNumber(443)) // port-2
						})},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-4"
							backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
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
							backendRef.Name = "service-5"
						})},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-5"
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
								backendRef.Name = "service-5"
							}),
							buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
								backendRef.Name = "service-6"
							}),
						},
					},
					{ // rule-2
						BackendRefs: []gwapiv1.HTTPBackendRef{buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
							backendRef.Name = "service-6"
							backendRef.Port = ptr.To(gwapiv1.PortNumber(80)) // port-1
						})},
					},
				}
			}),
			buildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
				r.Name = "route-7"
				r.Spec.ParentRefs[0].Name = "gateway-5"
				r.Spec.Rules[0].BackendRefs[0] = buildHTTPBackendRef(func(backendRef *gwapiv1.BackendObjectReference) {
					backendRef.Name = "service-7"
				})
			}),
		},
		services: []*core.Service{
			buildService(func(s *core.Service) {
				s.Name = "service-1"
				s.Spec.Ports[0].Name = "port-1"
				s.Spec.Ports = append(s.Spec.Ports, core.ServicePort{
					Name: "port-2",
					Port: 443,
				})
			}),
			buildService(func(s *core.Service) {
				s.Name = "service-2"
				s.Spec.Ports[0].Name = "port-1"
			}),
			buildService(func(s *core.Service) {
				s.Name = "service-3"
				s.Spec.Ports[0].Name = "port-1"
				s.Spec.Ports = append(s.Spec.Ports, core.ServicePort{
					Name: "port-2",
					Port: 443,
				})
			}),
			buildService(func(s *core.Service) {
				s.Name = "service-4"
				s.Spec.Ports[0].Name = "port-1"
			}),
			buildService(func(s *core.Service) {
				s.Name = "service-5"
				s.Spec.Ports[0].Name = "port-1"
			}),
			buildService(func(s *core.Service) {
				s.Name = "service-6"
				s.Spec.Ports[0].Name = "port-1"
			}),
			buildService(func(s *core.Service) {
				s.Name = "service-7"
				s.Spec.Ports[0].Name = "port-1"
			}),
		},
	}
	for _, opt := range opts {
		opt(&tc)
	}
	return tc
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

func buildService(f ...func(*core.Service)) *core.Service {
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
			TargetRef: gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
				LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
					Kind: "Service",
					Name: "my-service",
				},
			},
		},
	}
	for _, fn := range f {
		fn(p)
	}
	return p
}

func mapTargetableToURLFunc(t Targetable, _ int) string {
	return t.GetURL()
}

func listenersFromGatewayFunc(gateway Gateway, _ int) []Listener {
	return lo.Map(gateway.Spec.Listeners, func(listener gwapiv1.Listener, _ int) Listener {
		return Listener{
			Listener: &listener,
			gateway:  &gateway,
		}
	})
}

func httpRouteRulesFromHTTPRouteFunc(httpRoute HTTPRoute, _ int) []HTTPRouteRule {
	return lo.Map(httpRoute.Spec.Rules, func(rule gwapiv1.HTTPRouteRule, i int) HTTPRouteRule {
		return HTTPRouteRule{
			HTTPRouteRule: &rule,
			httpRoute:     &httpRoute,
			name:          gwapiv1.SectionName(fmt.Sprintf("rule-%d", i+1)),
		}
	})
}

func servicePortsFromBackendFunc(service Service, _ int) []ServicePort {
	return lo.Map(service.Spec.Ports, func(port core.ServicePort, _ int) ServicePort {
		return ServicePort{
			ServicePort: &port,
			service:     &service,
		}
	})
}

func linkGatewayClassToGatewayFunc(gatewayClasses []GatewayClass) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "GatewayClass"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		Func: func(child Targetable) []Targetable {
			gateway := child.(Gateway)
			gatewayClass, ok := lo.Find(gatewayClasses, func(gc GatewayClass) bool {
				return gc.Name == string(gateway.Spec.GatewayClassName)
			})
			if ok {
				return []Targetable{gatewayClass}
			}
			return nil
		},
	}
}

func linkGatewayToHTTPRouteFunc(gateways []Gateway) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		Func: func(child Targetable) []Targetable {
			httpRoute := child.(HTTPRoute)
			return lo.FilterMap(httpRoute.Spec.ParentRefs, func(parentRef gwapiv1.ParentReference, _ int) (Targetable, bool) {
				parentRefGroup := ptr.Deref(parentRef.Group, gwapiv1.Group(gwapiv1.GroupName))
				parentRefKind := ptr.Deref(parentRef.Kind, gwapiv1.Kind("Gateway"))
				if parentRefGroup != gwapiv1.GroupName || parentRefKind != "Gateway" {
					return nil, false
				}
				gatewayNamespace := string(ptr.Deref(parentRef.Namespace, gwapiv1.Namespace(httpRoute.Namespace)))
				return lo.Find(gateways, func(g Gateway) bool {
					return g.Namespace == gatewayNamespace && g.Name == string(parentRef.Name)
				})
			})
		},
	}
}

func linkGatewayToListenerFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Gateway"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Listener"},
		Func: func(child Targetable) []Targetable {
			listener := child.(Listener)
			return []Targetable{listener.gateway}
		},
	}
}

func linkListenerToHTTPRouteFunc(gateways []Gateway, listeners []Listener) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "Listener"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		Func: func(child Targetable) []Targetable {
			httpRoute := child.(HTTPRoute)
			return lo.FlatMap(httpRoute.Spec.ParentRefs, func(parentRef gwapiv1.ParentReference, _ int) []Targetable {
				parentRefGroup := ptr.Deref(parentRef.Group, gwapiv1.Group(gwapiv1.GroupName))
				parentRefKind := ptr.Deref(parentRef.Kind, gwapiv1.Kind("Gateway"))
				if parentRefGroup != gwapiv1.GroupName || parentRefKind != "Gateway" {
					return nil
				}
				gatewayNamespace := string(ptr.Deref(parentRef.Namespace, gwapiv1.Namespace(httpRoute.Namespace)))
				gateway, ok := lo.Find(gateways, func(g Gateway) bool {
					return g.Namespace == gatewayNamespace && g.Name == string(parentRef.Name)
				})
				if !ok {
					return nil
				}
				if parentRef.SectionName != nil {
					listener, ok := lo.Find(listeners, func(l Listener) bool {
						return l.gateway.GetURL() == gateway.GetURL() && l.Name == *parentRef.SectionName
					})
					if !ok {
						return nil
					}
					return []Targetable{listener}
				}
				return lo.FilterMap(listeners, func(l Listener, _ int) (Targetable, bool) {
					return l, l.gateway.GetURL() == gateway.GetURL()
				})
			})
		},
	}
}

func linkHTTPRouteToServiceFunc(httpRoutes []HTTPRoute) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		To:   schema.GroupKind{Kind: "Service"},
		Func: func(child Targetable) []Targetable {
			service := child.(Service)
			return lo.FilterMap(httpRoutes, func(httpRoute HTTPRoute, _ int) (Targetable, bool) {
				return httpRoute, lo.ContainsBy(httpRoute.Spec.Rules, func(rule gwapiv1.HTTPRouteRule) bool {
					backendRefs := lo.Map(rule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) gwapiv1.BackendRef { return backendRef.BackendRef })
					return lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, httpRoute.Namespace))
				})
			})
		},
	}
}

func linkHTTPRouteToHTTPRouteRuleFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRoute"},
		To:   schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		Func: func(child Targetable) []Targetable {
			httpRouteRule := child.(HTTPRouteRule)
			return []Targetable{httpRouteRule.httpRoute}
		},
	}
}

func linkHTTPRouteRuleToServiceFunc(httpRouteRules []HTTPRouteRule) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		To:   schema.GroupKind{Kind: "Service"},
		Func: func(child Targetable) []Targetable {
			service := child.(Service)
			return lo.FilterMap(httpRouteRules, func(httpRouteRule HTTPRouteRule, _ int) (Targetable, bool) {
				backendRefs := lo.FilterMap(httpRouteRule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) (gwapiv1.BackendRef, bool) {
					return backendRef.BackendRef, backendRef.Port == nil
				})
				return httpRouteRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(service, httpRouteRule.httpRoute.Namespace))
			})
		},
	}
}

func linkHTTPRouteRuleToServicePortFunc(httpRouteRules []HTTPRouteRule) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: gwapiv1.GroupVersion.Group, Kind: "HTTPRouteRule"},
		To:   schema.GroupKind{Kind: "ServicePort"},
		Func: func(child Targetable) []Targetable {
			servicePort := child.(ServicePort)
			return lo.FilterMap(httpRouteRules, func(httpRouteRule HTTPRouteRule, _ int) (Targetable, bool) {
				backendRefs := lo.FilterMap(httpRouteRule.BackendRefs, func(backendRef gwapiv1.HTTPBackendRef, _ int) (gwapiv1.BackendRef, bool) {
					return backendRef.BackendRef, backendRef.Port != nil && int32(*backendRef.Port) == servicePort.Port
				})
				return httpRouteRule, lo.ContainsBy(backendRefs, backendRefContainsServiceFunc(*servicePort.service, httpRouteRule.httpRoute.Namespace))
			})
		},
	}
}

func linkServiceToServicePortFunc() LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Kind: "Service"},
		To:   schema.GroupKind{Kind: "ServicePort"},
		Func: func(child Targetable) []Targetable {
			servicePort := child.(ServicePort)
			return []Targetable{servicePort.service}
		},
	}
}

func backendRefContainsServiceFunc(service Service, defaultNamespace string) func(backendRef gwapiv1.BackendRef) bool {
	return func(backendRef gwapiv1.BackendRef) bool {
		return backendRefEqualToService(backendRef, service, defaultNamespace)
	}
}

func backendRefEqualToService(backendRef gwapiv1.BackendRef, service Service, defaultNamespace string) bool {
	backendRefGroup := string(ptr.Deref(backendRef.Group, gwapiv1.Group("")))
	backendRefKind := string(ptr.Deref(backendRef.Kind, gwapiv1.Kind("Service")))
	backendRefNamespace := string(ptr.Deref(backendRef.Namespace, gwapiv1.Namespace(defaultNamespace)))
	return backendRefGroup == service.GroupVersionKind().Group && backendRefKind == service.GroupVersionKind().Kind && backendRefNamespace == service.Namespace && string(backendRef.Name) == service.Name
}

func linksFromNode(topology *Topology, node Targetable, edges map[string][]string) {
	if _, ok := edges[node.GetName()]; ok {
		return
	}
	children := topology.Children(node)
	edges[node.GetName()] = lo.Map(children, func(child Targetable, _ int) string { return child.GetName() })
	for _, child := range children {
		linksFromNode(topology, child, edges)
	}
}

// saveTestCaseOutput saves the output of a test case to a file in the output directory.
func saveTestCaseOutput(t *testing.T, out *bytes.Buffer) {
	file, err := os.Create(fmt.Sprintf("%s/%s.dot", outDir, strings.ReplaceAll(t.Name(), "/", "__")))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, err = file.Write(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
}
