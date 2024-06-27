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
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const outDir = "../tests/out"

func TestTopologyRoots(t *testing.T) {
	gateways := []*Gateway{
		{Gateway: BuildGateway()},
		{Gateway: BuildGateway(func(g *gwapiv1.Gateway) { g.Name = "my-gateway-2" })},
	}
	httpRoute := &HTTPRoute{HTTPRoute: BuildHTTPRoute()}
	httpRouteRules := HTTPRouteRulesFromHTTPRouteFunc(httpRoute, 0)
	topology := NewTopology(
		WithTargetables(gateways...),
		WithTargetables(httpRoute),
		WithTargetables(httpRouteRules...),
		WithLinks(
			LinkGatewayToHTTPRouteFunc(gateways),
			LinkHTTPRouteToHTTPRouteRuleFunc(),
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
	rootURLs := lo.Map(roots, MapTargetableToURLFunc)
	for _, gateway := range gateways {
		if !lo.Contains(rootURLs, gateway.GetURL()) {
			t.Errorf("expected root %s not found", gateway.GetURL())
		}
	}
}

func TestTopologyParents(t *testing.T) {
	gateways := []*Gateway{{Gateway: BuildGateway()}}
	httpRoute := &HTTPRoute{HTTPRoute: BuildHTTPRoute()}
	httpRouteRules := HTTPRouteRulesFromHTTPRouteFunc(httpRoute, 0)
	topology := NewTopology(
		WithTargetables(gateways...),
		WithTargetables(httpRoute),
		WithTargetables(httpRouteRules...),
		WithLinks(
			LinkGatewayToHTTPRouteFunc(gateways),
			LinkHTTPRouteToHTTPRouteRuleFunc(),
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
	parentURLs := lo.Map(parents, MapTargetableToURLFunc)
	for _, gateway := range gateways {
		if !lo.Contains(parentURLs, gateway.GetURL()) {
			t.Errorf("expected parent %s not found", gateway.GetURL())
		}
	}
}

func TestTopologyChildren(t *testing.T) {
	gateways := []*Gateway{{Gateway: BuildGateway()}}
	httpRoute := &HTTPRoute{HTTPRoute: BuildHTTPRoute()}
	httpRouteRules := HTTPRouteRulesFromHTTPRouteFunc(httpRoute, 0)
	topology := NewTopology(
		WithTargetables(gateways...),
		WithTargetables(httpRoute),
		WithTargetables(httpRouteRules...),
		WithLinks(
			LinkGatewayToHTTPRouteFunc(gateways),
			LinkHTTPRouteToHTTPRouteRuleFunc(),
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
	childURLs := lo.Map(children, MapTargetableToURLFunc)
	for _, httpRouteRule := range httpRouteRules {
		if !lo.Contains(childURLs, httpRouteRule.GetURL()) {
			t.Errorf("expected child %s not found", httpRouteRule.GetURL())
		}
	}
}

func TestTopologyPaths(t *testing.T) {
	gateways := []*Gateway{{Gateway: BuildGateway()}}
	httpRoutes := []*HTTPRoute{
		{HTTPRoute: BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) { r.Name = "route-1" })},
		{HTTPRoute: BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) { r.Name = "route-2" })},
	}
	services := []*Service{{Service: BuildService()}}
	topology := NewTopology(
		WithTargetables(gateways...),
		WithTargetables(httpRoutes...),
		WithTargetables(services...),
		WithLinks(
			LinkGatewayToHTTPRouteFunc(gateways),
			LinkHTTPRouteToServiceFunc(httpRoutes),
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
				return strings.Join(lo.Map(expectedPath, MapTargetableToURLFunc), "→")
			})
			for _, path := range paths {
				pathString := strings.Join(lo.Map(path, MapTargetableToURLFunc), "→")
				if !lo.Contains(expectedPaths, pathString) {
					t.Errorf("expected path %v not found", pathString)
				}
			}
		})
	}
}

// TestGatewayAPITopology tests for a topology of Gateway API resources with the following architecture:
//
//	GatewayClass -> Gateway -> Listener -> HTTPRoute -> HTTPRouteRule -> Service -> ServicePort
//	                                                                  ∟> ServicePort <- Service
func TestGatewayAPITopology(t *testing.T) {
	testCases := []struct {
		name          string
		targetables   GatewayAPIResources
		policies      []Policy
		expectedLinks map[string][]string
	}{
		{
			name: "empty",
		},
		{
			name: "single node",
			targetables: GatewayAPIResources{
				GatewayClasses: []*gwapiv1.GatewayClass{BuildGatewayClass()},
			},
		},
		{
			name: "one of each kind",
			targetables: GatewayAPIResources{
				GatewayClasses: []*gwapiv1.GatewayClass{BuildGatewayClass()},
				Gateways:       []*gwapiv1.Gateway{BuildGateway()},
				HTTPRoutes:     []*gwapiv1.HTTPRoute{BuildHTTPRoute()},
				Services:       []*core.Service{BuildService()},
			},
			policies: []Policy{buildPolicy()},
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
			targetables: GatewayAPIResources{
				Gateways: []*gwapiv1.Gateway{BuildGateway(func(gateway *gwapiv1.Gateway) {
					gateway.Spec.Listeners[0].Name = "http"
					gateway.Spec.Listeners = append(gateway.Spec.Listeners, gwapiv1.Listener{
						Name:     "https",
						Port:     443,
						Protocol: "HTTPS",
					})
				})},
			},
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
		{
			name:        "complex topology",
			targetables: BuildComplexGatewayAPITopology(),
			expectedLinks: map[string][]string{
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
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gatewayClasses := lo.Map(tc.targetables.GatewayClasses, func(gatewayClass *gwapiv1.GatewayClass, _ int) *GatewayClass {
				return &GatewayClass{GatewayClass: gatewayClass}
			})
			gateways := lo.Map(tc.targetables.Gateways, func(gateway *gwapiv1.Gateway, _ int) *Gateway { return &Gateway{Gateway: gateway} })
			listeners := lo.FlatMap(gateways, ListenersFromGatewayFunc)
			httpRoutes := lo.Map(tc.targetables.HTTPRoutes, func(httpRoute *gwapiv1.HTTPRoute, _ int) *HTTPRoute { return &HTTPRoute{HTTPRoute: httpRoute} })
			httpRouteRules := lo.FlatMap(httpRoutes, HTTPRouteRulesFromHTTPRouteFunc)
			services := lo.Map(tc.targetables.Services, func(service *core.Service, _ int) *Service { return &Service{Service: service} })
			servicePorts := lo.FlatMap(services, ServicePortsFromBackendFunc)

			topology := NewTopology(
				WithTargetables(gatewayClasses...),
				WithTargetables(gateways...),
				WithTargetables(listeners...),
				WithTargetables(httpRoutes...),
				WithTargetables(httpRouteRules...),
				WithTargetables(services...),
				WithTargetables(servicePorts...),
				WithLinks(
					LinkGatewayClassToGatewayFunc(gatewayClasses),
					LinkGatewayToListenerFunc(),
					LinkListenerToHTTPRouteFunc(gateways, listeners),
					LinkHTTPRouteToHTTPRouteRuleFunc(),
					LinkHTTPRouteRuleToServiceFunc(httpRouteRules),
					LinkHTTPRouteRuleToServicePortFunc(httpRouteRules),
					LinkServiceToServicePortFunc(),
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
	testCases := []struct {
		name          string
		targetables   GatewayAPIResources
		policies      []Policy
		expectedLinks map[string][]string
	}{
		{
			name: "one of each kind",
			targetables: GatewayAPIResources{
				GatewayClasses: []*gwapiv1.GatewayClass{BuildGatewayClass()},
				Gateways:       []*gwapiv1.Gateway{BuildGateway()},
				HTTPRoutes:     []*gwapiv1.HTTPRoute{BuildHTTPRoute()},
				Services:       []*core.Service{BuildService()},
			},
			policies: []Policy{buildPolicy()},
			expectedLinks: map[string][]string{
				"my-gateway-class": {"my-gateway"},
				"my-gateway":       {"my-http-route"},
				"my-http-route":    {"my-service"},
			},
		},
		{
			name:        "complex topology",
			targetables: BuildComplexGatewayAPITopology(),
			expectedLinks: map[string][]string{
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
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gatewayClasses := lo.Map(tc.targetables.GatewayClasses, func(gatewayClass *gwapiv1.GatewayClass, _ int) *GatewayClass {
				return &GatewayClass{GatewayClass: gatewayClass}
			})
			gateways := lo.Map(tc.targetables.Gateways, func(gateway *gwapiv1.Gateway, _ int) *Gateway { return &Gateway{Gateway: gateway} })
			httpRoutes := lo.Map(tc.targetables.HTTPRoutes, func(httpRoute *gwapiv1.HTTPRoute, _ int) *HTTPRoute { return &HTTPRoute{HTTPRoute: httpRoute} })
			services := lo.Map(tc.targetables.Services, func(service *core.Service, _ int) *Service { return &Service{Service: service} })

			topology := NewTopology(
				WithTargetables(gatewayClasses...),
				WithTargetables(gateways...),
				WithTargetables(httpRoutes...),
				WithTargetables(services...),
				WithLinks(
					LinkGatewayClassToGatewayFunc(gatewayClasses),
					LinkGatewayToHTTPRouteFunc(gateways),
					LinkHTTPRouteToServiceFunc(httpRoutes),
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
