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
//	                        ∟> GRPCRoute ⤴
//	                        ∟> TCPRoute  ⤴
//	                        ∟> TLSRoute  ⤴
//	                        ∟> UDPRoute  ⤴
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
				TCPRoutes:      []*gwapiv1alpha2.TCPRoute{BuildTCPRoute()},
				TLSRoutes:      []*gwapiv1alpha2.TLSRoute{BuildTLSRoute()},
				UDPRoutes:      []*gwapiv1alpha2.UDPRoute{BuildUDPRoute()},
				Services:       []*core.Service{BuildService()},
			},
			policies: []Policy{buildPolicy()},
			expectedLinks: map[string][]string{
				"my-gateway-class": {"my-gateway"},
				"my-gateway":       {"my-http-route", "my-grpc-route", "my-tcp-route", "my-tls-route", "my-udp-route"},
				"my-grpc-route":    {"my-service"},
				"my-http-route":    {"my-service"},
				"my-tcp-route":     {"my-service"},
				"my-tls-route":     {"my-service"},
				"my-udp-route":     {"my-service"},
			},
		},
		{
			name:        "complex topology",
			targetables: BuildComplexGatewayAPITopology(),
			expectedLinks: map[string][]string{
				"gatewayclass-1": {"gateway-1", "gateway-2", "gateway-3"},
				"gatewayclass-2": {"gateway-4", "gateway-5"},
				"gateway-1":      {"http-route-1", "http-route-2"},
				"gateway-2":      {"http-route-2", "http-route-3"},
				"gateway-3":      {"udp-route-1", "tls-route-1"},
				"gateway-4":      {"tls-route-1", "tcp-route-1"},
				"gateway-5":      {"grpc-route-1"},
				"grpc-route-1":   {"service-7"},
				"http-route-1":   {"service-1", "service-2"},
				"http-route-2":   {"service-3"},
				"http-route-3":   {"service-3"},
				"udp-route-1":    {"service-3", "service-4"},
				"tls-route-1":    {"service-5"},
				"tcp-route-1":    {"service-5", "service-6"},
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
			tcpRoutes := lo.Map(tc.targetables.TCPRoutes, func(tcpRoute *gwapiv1alpha2.TCPRoute, _ int) *TCPRoute { return &TCPRoute{TCPRoute: tcpRoute} })
			tlsRoutes := lo.Map(tc.targetables.TLSRoutes, func(tlsRoute *gwapiv1alpha2.TLSRoute, _ int) *TLSRoute { return &TLSRoute{TLSRoute: tlsRoute} })
			udpRoutes := lo.Map(tc.targetables.UDPRoutes, func(updRoute *gwapiv1alpha2.UDPRoute, _ int) *UDPRoute { return &UDPRoute{UDPRoute: updRoute} })
			services := lo.Map(tc.targetables.Services, func(service *core.Service, _ int) *Service { return &Service{Service: service} })

			topology := NewTopology(
				WithTargetables(gatewayClasses...),
				WithTargetables(gateways...),
				WithTargetables(httpRoutes...),
				WithTargetables(services...),
				WithTargetables(grpcRoutes...),
				WithTargetables(tcpRoutes...),
				WithTargetables(tlsRoutes...),
				WithTargetables(udpRoutes...),
				WithLinks(
					LinkGatewayClassToGatewayFunc(gatewayClasses),
					LinkGatewayToHTTPRouteFunc(gateways),
					LinkGatewayToGRPCRouteFunc(gateways),
					LinkGatewayToTCPRouteFunc(gateways),
					LinkGatewayToTLSRouteFunc(gateways),
					LinkGatewayToUDPRouteFunc(gateways),
					LinkHTTPRouteToServiceFunc(httpRoutes, false),
					LinkGRPCRouteToServiceFunc(grpcRoutes, false),
					LinkTCPRouteToServiceFunc(tcpRoutes, false),
					LinkTLSRouteToServiceFunc(tlsRoutes, false),
					LinkUDPRouteToServiceFunc(udpRoutes, false),
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
				TCPRoutes:      []*gwapiv1alpha2.TCPRoute{BuildTCPRoute()},
				TLSRoutes:      []*gwapiv1alpha2.TLSRoute{BuildTLSRoute()},
				UDPRoutes:      []*gwapiv1alpha2.UDPRoute{BuildUDPRoute()},
				Services:       []*core.Service{BuildService()},
			},
			policies: []Policy{buildPolicy()},
			expectedLinks: map[string][]string{
				"my-gateway-class":       {"my-gateway"},
				"my-gateway":             {"my-gateway#my-listener"},
				"my-gateway#my-listener": {"my-http-route", "my-grpc-route", "my-tcp-route", "my-tls-route", "my-udp-route"},
				"my-grpc-route":          {"my-grpc-route#rule-1"},
				"my-grpc-route#rule-1":   {"my-service"},
				"my-http-route":          {"my-http-route#rule-1"},
				"my-http-route#rule-1":   {"my-service"},
				"my-tcp-route":           {"my-tcp-route#rule-1"},
				"my-tcp-route#rule-1":    {"my-service"},
				"my-tls-route":           {"my-tls-route#rule-1"},
				"my-tls-route#rule-1":    {"my-service"},
				"my-udp-route":           {"my-udp-route#rule-1"},
				"my-udp-route#rule-1":    {"my-service"},
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
				"gateway-1#listener-1": {"http-route-1"},
				"gateway-1#listener-2": {"http-route-1", "http-route-2"},
				"gateway-2#listener-1": {"http-route-2", "http-route-3"},
				"gateway-3#listener-1": {"udp-route-1", "tls-route-1"},
				"gateway-3#listener-2": {"udp-route-1", "tls-route-1"},
				"gateway-4#listener-1": {"tls-route-1", "tcp-route-1"},
				"gateway-4#listener-2": {"tls-route-1", "tcp-route-1"},
				"gateway-5#listener-1": {"grpc-route-1"},
				"grpc-route-1":         {"grpc-route-1#rule-1"},
				"grpc-route-1#rule-1":  {"service-7"},
				"http-route-1":         {"http-route-1#rule-1", "http-route-1#rule-2"},
				"http-route-2":         {"http-route-2#rule-1"},
				"http-route-3":         {"http-route-3#rule-1"},
				"udp-route-1":          {"udp-route-1#rule-1", "udp-route-1#rule-2"},
				"http-route-1#rule-1":  {"service-1"},
				"http-route-1#rule-2":  {"service-2"},
				"http-route-2#rule-1":  {"service-3#port-1"},
				"http-route-3#rule-1":  {"service-3#port-1"},
				"udp-route-1#rule-1":   {"service-3#port-2"},
				"udp-route-1#rule-2":   {"service-4#port-1"},
				"tls-route-1":          {"tls-route-1#rule-1", "tls-route-1#rule-2"},
				"tls-route-1#rule-1":   {"service-5"},
				"tls-route-1#rule-2":   {"service-5"},
				"tcp-route-1":          {"tcp-route-1#rule-1", "tcp-route-1#rule-2"},
				"tcp-route-1#rule-1":   {"service-5", "service-6"},
				"tcp-route-1#rule-2":   {"service-6#port-1"},
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
				WithTCPRoutes(tc.targetables.TCPRoutes...),
				ExpandTCPRouteRules(),
				WithTLSRoutes(tc.targetables.TLSRoutes...),
				ExpandTLSRouteRules(),
				WithServices(tc.targetables.Services...),
				WithUDPRoutes(tc.targetables.UDPRoutes...),
				ExpandUDPRouteRules(),
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
