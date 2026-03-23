//go:build unit

package controller

import (
	"testing"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/policy-machinery/machinery"
)

func TestGatewayAPITopologyBuilder(t *testing.T) {
	testCases := []struct {
		name               string
		objects            []Object
		expectedGateways   int
		expectedHTTPRoutes int
		expectedGRPCRoutes int
		expectedServices   int
	}{
		{
			name:               "empty store",
			objects:            []Object{},
			expectedGateways:   0,
			expectedHTTPRoutes: 0,
			expectedGRPCRoutes: 0,
			expectedServices:   0,
		},
		{
			name: "gateway only",
			objects: []Object{
				machinery.BuildGateway(func(g *gwapiv1.Gateway) {
					g.UID = types.UID("gateway-1")
				}),
			},
			expectedGateways:   1,
			expectedHTTPRoutes: 0,
			expectedGRPCRoutes: 0,
			expectedServices:   0,
		},
		{
			name: "httproute only",
			objects: []Object{
				machinery.BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
					r.UID = types.UID("httproute-1")
				}),
			},
			expectedGateways:   0,
			expectedHTTPRoutes: 1,
			expectedGRPCRoutes: 0,
			expectedServices:   0,
		},
		{
			name: "grpcroute only",
			objects: []Object{
				machinery.BuildGRPCRoute(func(r *gwapiv1.GRPCRoute) {
					r.UID = types.UID("grpcroute-1")
				}),
			},
			expectedGateways:   0,
			expectedHTTPRoutes: 0,
			expectedGRPCRoutes: 1,
			expectedServices:   0,
		},
		{
			name: "service only",
			objects: []Object{
				machinery.BuildService(func(s *corev1.Service) {
					s.UID = types.UID("service-1")
				}),
			},
			expectedGateways:   0,
			expectedHTTPRoutes: 0,
			expectedGRPCRoutes: 0,
			expectedServices:   1,
		},
		{
			name: "mixed resources",
			objects: []Object{
				machinery.BuildGateway(func(g *gwapiv1.Gateway) {
					g.UID = types.UID("gateway-1")
				}),
				machinery.BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
					r.UID = types.UID("httproute-1")
				}),
				machinery.BuildGRPCRoute(func(r *gwapiv1.GRPCRoute) {
					r.UID = types.UID("grpcroute-1")
				}),
				machinery.BuildService(func(s *corev1.Service) {
					s.UID = types.UID("service-1")
				}),
			},
			expectedGateways:   1,
			expectedHTTPRoutes: 1,
			expectedGRPCRoutes: 1,
			expectedServices:   1,
		},
		{
			name: "multiple grpcroutes",
			objects: []Object{
				machinery.BuildGRPCRoute(func(r *gwapiv1.GRPCRoute) {
					r.Name = "grpcroute-1"
					r.UID = types.UID("grpcroute-1")
				}),
				machinery.BuildGRPCRoute(func(r *gwapiv1.GRPCRoute) {
					r.Name = "grpcroute-2"
					r.UID = types.UID("grpcroute-2")
				}),
				machinery.BuildGRPCRoute(func(r *gwapiv1.GRPCRoute) {
					r.Name = "grpcroute-3"
					r.Namespace = "app"
					r.UID = types.UID("grpcroute-3")
				}),
			},
			expectedGateways:   0,
			expectedHTTPRoutes: 0,
			expectedGRPCRoutes: 3,
			expectedServices:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := Store{}
			for _, obj := range tc.objects {
				store[string(obj.GetUID())] = obj
			}

			builder := newGatewayAPITopologyBuilder(nil, nil, nil, false)
			topology, err := builder.Build(store)

			if err != nil {
				t.Fatalf("unexpected error building topology: %v", err)
			}

			if topology == nil {
				t.Fatal("expected topology, got nil")
			}

			// Count objects by kind in the topology - use Targetables for route objects
			targetables := topology.Targetables().Items()
			gateways := lo.Filter(targetables, func(obj machinery.Targetable, _ int) bool {
				return obj.GroupVersionKind().Kind == "Gateway"
			})
			httpRoutes := lo.Filter(targetables, func(obj machinery.Targetable, _ int) bool {
				return obj.GroupVersionKind().Kind == "HTTPRoute"
			})
			grpcRoutes := lo.Filter(targetables, func(obj machinery.Targetable, _ int) bool {
				return obj.GroupVersionKind().Kind == "GRPCRoute"
			})
			services := lo.Filter(targetables, func(obj machinery.Targetable, _ int) bool {
				return obj.GroupVersionKind().Kind == "Service"
			})

			if len(gateways) != tc.expectedGateways {
				t.Errorf("expected %d gateways in topology, got %d", tc.expectedGateways, len(gateways))
			}
			if len(httpRoutes) != tc.expectedHTTPRoutes {
				t.Errorf("expected %d httproutes in topology, got %d", tc.expectedHTTPRoutes, len(httpRoutes))
			}
			if len(grpcRoutes) != tc.expectedGRPCRoutes {
				t.Errorf("expected %d grpcroutes in topology, got %d", tc.expectedGRPCRoutes, len(grpcRoutes))
			}
			if len(services) != tc.expectedServices {
				t.Errorf("expected %d services in topology, got %d", tc.expectedServices, len(services))
			}
		})
	}
}

func TestGatewayAPITopologyBuilder_GRPCRouteWithMultipleRules(t *testing.T) {
	// Test that GRPCRoutes with multiple rules are correctly passed to the machinery layer
	grpcRoute := machinery.BuildGRPCRoute(func(r *gwapiv1.GRPCRoute) {
		r.UID = types.UID("grpcroute-1")
		// Adding backend refs is important for the machinery layer to properly build the topology
		r.Spec.Rules = []gwapiv1.GRPCRouteRule{
			{
				Matches: []gwapiv1.GRPCRouteMatch{
					{
						Method: &gwapiv1.GRPCMethodMatch{
							Service: ptr.To("example.Service"),
							Method:  ptr.To("Echo"),
						},
					},
				},
				BackendRefs: []gwapiv1.GRPCBackendRef{machinery.BuildGRPCBackendRef()},
			},
			{
				Matches: []gwapiv1.GRPCRouteMatch{
					{
						Method: &gwapiv1.GRPCMethodMatch{
							Service: ptr.To("example.Service"),
							Method:  ptr.To("Stream"),
						},
					},
				},
				BackendRefs: []gwapiv1.GRPCBackendRef{machinery.BuildGRPCBackendRef()},
			},
		}
	})

	store := Store{
		string(grpcRoute.GetUID()): grpcRoute,
	}

	builder := newGatewayAPITopologyBuilder(nil, nil, nil, false)
	topology, err := builder.Build(store)

	if err != nil {
		t.Fatalf("unexpected error building topology: %v", err)
	}

	// Verify the GRPCRoute is present in the topology
	targetables := topology.Targetables().Items()
	grpcRoutes := lo.Filter(targetables, func(obj machinery.Targetable, _ int) bool {
		return obj.GroupVersionKind().Kind == "GRPCRoute"
	})

	if len(grpcRoutes) != 1 {
		t.Fatalf("expected 1 GRPCRoute in topology, got %d", len(grpcRoutes))
	}

	// Verify that ExpandGRPCRouteRules() was called by checking for rule objects
	grpcRouteRules := lo.Filter(targetables, func(obj machinery.Targetable, _ int) bool {
		return obj.GroupVersionKind().Kind == "GRPCRouteRule"
	})

	if len(grpcRouteRules) != 2 {
		t.Errorf("expected 2 GRPCRouteRule objects after ExpandGRPCRouteRules(), got %d", len(grpcRouteRules))
	}

	// The machinery layer is responsible for the actual expansion logic,
	// which is tested in machinery/gateway_api_topology_test.go.
	// Here we verify that the controller layer correctly calls ExpandGRPCRouteRules().
}
