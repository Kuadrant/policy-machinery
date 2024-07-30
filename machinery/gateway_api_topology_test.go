//go:build unit

package machinery

import (
	"slices"
	"testing"

	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// TestGatewayAPITopology tests for a simplified topology of Gateway API resources without section names,
// i.e. where HTTPRoutes are not expanded to link from Listeners, and Policy TargetRefs are not of
// LocalPolicyTargetReferenceWithSectionName kind.
//
// This results in a topology with the following scheme:
//
//	GatewayClass -> Gateway -> HTTPRoute -> Service
//							∟> GRPCRoute ⤴
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
				GRPCRoutes:     []*gwapiv1.GRPCRoute{BuildGRPCRoute()},
				Services:       []*core.Service{BuildService()},
			},
			policies: []Policy{buildPolicy()},
			expectedLinks: map[string][]string{
				"my-gateway-class": {"my-gateway"},
				"my-gateway":       {"my-http-route", "my-grpc-route"},
				"my-grpc-route":    {"my-service"},
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
			grpcRoutes := lo.Map(tc.targetables.GRPCRoutes, func(grpcRoute *gwapiv1.GRPCRoute, _ int) *GRPCRoute { return &GRPCRoute{GRPCRoute: grpcRoute} })
			services := lo.Map(tc.targetables.Services, func(service *core.Service, _ int) *Service { return &Service{Service: service} })

			topology := NewTopology(
				WithTargetables(gatewayClasses...),
				WithTargetables(gateways...),
				WithTargetables(httpRoutes...),
				WithTargetables(services...),
				WithTargetables(grpcRoutes...),
				WithLinks(
					LinkGatewayClassToGatewayFunc(gatewayClasses),
					LinkGatewayToHTTPRouteFunc(gateways),
					LinkHTTPRouteToServiceFunc(httpRoutes, false),
					LinkGatewayToGRPCRouteFunc(gateways),
					LinkGRPCRouteToServiceFunc(grpcRoutes, false),
				),
				WithPolicies(tc.policies...),
			)

			links := make(map[string][]string)
			for _, root := range topology.Targetables().Roots() {
				linksFromTargetable(topology, root, links)
			}
			for from, tos := range links {
				expectedTos := tc.expectedLinks[from]
				slices.Sort(expectedTos)
				slices.Sort(tos)
				if !slices.Equal(expectedTos, tos) {
					t.Errorf("expected links from %s to be %v, got %v", from, expectedTos, tos)
				}
			}

			SaveToOutputDir(t, topology.ToDot(), "../tests/out", ".dot")
		})
	}
}

// TestGatewayAPITopologyWithSectionNames tests for a topology of Gateway API resources where Gateways, HTTPRoutes
// and Services are expanded to include their named sections as targetables in the topology.
//
// This results in a topology with the following scheme:
//
//	GatewayClass -> Gateway -> Listener -> HTTPRoute -> HTTPRouteRule -> Service -> ServicePort
//	                                                                  ∟> ServicePort <- Service
func TestGatewayAPITopologyWithSectionNames(t *testing.T) {
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
				GRPCRoutes:     []*gwapiv1.GRPCRoute{BuildGRPCRoute()},
				Services:       []*core.Service{BuildService()},
			},
			policies: []Policy{buildPolicy()},
			expectedLinks: map[string][]string{
				"my-gateway-class":       {"my-gateway"},
				"my-gateway":             {"my-gateway#my-listener"},
				"my-gateway#my-listener": {"my-http-route", "my-grpc-route"},
				"my-grpc-route":          {"my-grpc-route#rule-1"},
				"my-grpc-route#rule-1":   {"my-service"},
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
			topology := NewGatewayAPITopology(
				WithGatewayClasses(tc.targetables.GatewayClasses...),
				WithGateways(tc.targetables.Gateways...),
				ExpandGatewayListeners(),
				WithHTTPRoutes(tc.targetables.HTTPRoutes...),
				ExpandHTTPRouteRules(),
				WithGRPCRoutes(tc.targetables.GRPCRoutes...),
				ExpandGRPCRouteRules(),
				WithServices(tc.targetables.Services...),
				ExpandServicePorts(),
				WithGatewayAPITopologyPolicies(tc.policies...),
			)

			links := make(map[string][]string)
			for _, root := range topology.Targetables().Roots() {
				linksFromTargetable(topology, root, links)
			}
			for from, tos := range links {
				expectedTos := tc.expectedLinks[from]
				slices.Sort(expectedTos)
				slices.Sort(tos)
				if !slices.Equal(expectedTos, tos) {
					t.Errorf("expected links from %s to be %v, got %v", from, expectedTos, tos)
				}
			}

			SaveToOutputDir(t, topology.ToDot(), "../tests/out", ".dot")
		})
	}
}
