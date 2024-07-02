//go:build integration

package json_patch

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/machinery"
)

// TestJSONPatchMergeBasedOnTopology tests ColorPolicy's merge strategies for painting a house, based on network traffic
// flowing through the following topology of Gateway API resources:
//
//	                                                 ┌────────────┐
//	house-colors-gw ────────────────────────────────►│ my-gateway │
//	                                                 └────────────┘
//	                                                       ▲
//	                                                       │
//	                                          ┌────────────┴─────────────┐
//	                                          │                          │
//	                              ┌───────────┴───────────┐  ┌───────────┴───────────┐
//	house-colors-route-1 ────────►│       my-route-1      │  │       my-route-2      │
//	                              │                       │  │                       │
//	                              │ ┌────────┐ ┌────────┐ │  │ ┌────────┐ ┌────────┐ │
//	house-colors-route-1-rule-1 ──┤►│ rule-1 │ │ rule-2 │ │  │ │ rule-2 │ │ rule-1 │◄├──── house-colors-route-2-rule-1
//	                              │ └───┬────┘ └────┬───┘ │  │ └────┬───┘ └────┬───┘ │
//	                              │     │           │     │  │      │          │     │
//	                              └─────┼───────────┼─────┘  └──────┼──────────┼─────┘
//	                                    │           │               │          │
//	                                    └───────────┴───────┬───────┴──────────┘
//	                                                        │
//	                                                        ▼
//	                                                 ┌────────────┐
//	                                                 │ my-service │
//	                                                 └────────────┘
func TestJSONPatchMergeBasedOnTopology(t *testing.T) {
	gateway := machinery.BuildGateway()
	httpRoutes := []*gwapiv1.HTTPRoute{
		machinery.BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
			r.Name = "my-route-1"
			r.Spec.Rules = append(r.Spec.Rules, gwapiv1.HTTPRouteRule{
				BackendRefs: []gwapiv1.HTTPBackendRef{machinery.BuildHTTPBackendRef()},
			})
		}),
		machinery.BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
			r.Name = "my-route-2"
			r.Spec.Rules = append(r.Spec.Rules, gwapiv1.HTTPRouteRule{
				BackendRefs: []gwapiv1.HTTPBackendRef{machinery.BuildHTTPBackendRef()},
			})
		}),
	}
	services := []*core.Service{machinery.BuildService()}
	policies := []machinery.Policy{
		buildPolicy(func(p *ColorPolicy) { // atomic defaults
			p.Name = "house-colors-gw"
			p.Spec.TargetRef.Group = gwapiv1.GroupName
			p.Spec.TargetRef.Kind = "Gateway"
			p.Spec.TargetRef.Name = "my-gateway"
			p.Spec.ColorSpecProper = ColorSpecProper{}
			p.Spec.Defaults = &ColorSpecProper{
				Rules: map[string]ColorValue{
					"walls": Black,
					"doors": Blue,
				},
			}
		}),
		buildPolicy(func(p *ColorPolicy) { // policy rule overrides
			p.Name = "house-colors-route-1"
			p.Spec.TargetRef.Group = gwapiv1.GroupName
			p.Spec.TargetRef.Kind = "HTTPRoute"
			p.Spec.TargetRef.Name = "my-route-1"
			p.Spec.ColorSpecProper = ColorSpecProper{}
			p.Spec.Overrides = &ColorSpecProper{
				Rules: map[string]ColorValue{
					"walls": Green,
					"roof":  Orange,
				},
			}
		}),
		buildPolicy(func(p *ColorPolicy) { // default: atomic defaults
			p.Name = "house-colors-route-1-rule-1"
			p.Spec.TargetRef.Group = gwapiv1.GroupName
			p.Spec.TargetRef.Kind = "HTTPRoute"
			p.Spec.TargetRef.Name = "my-route-1"
			p.Spec.TargetRef.SectionName = ptr.To(gwapiv1.SectionName("rule-1"))
			p.Spec.Rules = map[string]ColorValue{
				"roof":  Purple,
				"floor": Red,
			}
		}),
		buildPolicy(func(p *ColorPolicy) { // default: atomic defaults
			p.Name = "house-colors-route-2-rule-1"
			p.Spec.TargetRef.Group = gwapiv1.GroupName
			p.Spec.TargetRef.Kind = "HTTPRoute"
			p.Spec.TargetRef.Name = "my-route-2"
			p.Spec.TargetRef.SectionName = ptr.To(gwapiv1.SectionName("rule-1"))
			p.Spec.Rules = map[string]ColorValue{
				"walls": White,
				"floor": Yellow,
			}
		}),
	}

	topology := machinery.NewGatewayAPITopology(
		machinery.WithGateways(gateway),
		machinery.WithHTTPRoutes(httpRoutes...),
		machinery.ExpandHTTPRouteRules(),
		machinery.WithServices(services...),
		machinery.WithGatewayAPITopologyPolicies(policies...),
	)

	machinery.SaveToOutputDir(t, topology.ToDot(), "../../tests/out", ".dot")

	gateways := topology.Targetables(func(o machinery.Object) bool {
		_, ok := o.(*machinery.Gateway)
		return ok
	})
	httpRouteRules := topology.Targetables(func(o machinery.Object) bool {
		_, ok := o.(*machinery.HTTPRouteRule)
		return ok
	})

	effectivePoliciesByPath := make(map[string]ColorPolicy)

	for _, httpRouteRule := range httpRouteRules {
		for _, path := range topology.Paths(gateways[0], httpRouteRule) {
			// Gather all policies in the path sorted from the least specific (gateway) to the most specific (httprouterule)
			// Since in this example there are no targetables with more than one policy attached to it, we can safely just
			// flat the slices of policies; otherwise we would need to ensure that the policies at the same level are sorted
			// by creationTimeStamp.
			policies := lo.FlatMap(path, func(targetable machinery.Targetable, _ int) []machinery.Policy {
				return targetable.Policies()
			})

			// Map reduces the policies from most specific to least specific, merging them into one effective policy for
			// each path
			var emptyPolicy machinery.Policy = buildPolicy()
			effectivePolicy := lo.ReduceRight(policies, func(effectivePolicy machinery.Policy, policy machinery.Policy, _ int) machinery.Policy {
				return effectivePolicy.Merge(policy)
			}, emptyPolicy)

			pathStr := strings.Join(lo.Map(path, func(t machinery.Targetable, _ int) string { return t.GetName() }), " → ")
			effectiveColorPolicy := effectivePolicy.(*ColorPolicy)
			effectivePoliciesByPath[pathStr] = *effectiveColorPolicy.DeepCopy()

			jsonPolicy, _ := json.MarshalIndent(effectivePolicy, "", "  ")
			fmt.Printf("Effective policy for path %s:\n%s\n", pathStr, jsonPolicy)
		}
	}

	expectedPolicyRulesByPath := map[string]map[string]ColorValue{
		"my-gateway → my-route-1 → my-route-1#rule-1": {
			// from house-colors-gw
			"doors": Blue,
			// from house-colors-route-1
			"walls": Green,
			"roof":  Orange,
			// from house-colors-route-1-rule-1
			"floor": Red,
		},
		"my-gateway → my-route-1 → my-route-1#rule-2": {
			// from house-colors-gw
			"doors": Blue,
			// from house-colors-route-1
			"walls": Green,
			"roof":  Orange,
		},
		"my-gateway → my-route-2 → my-route-2#rule-1": {
			// from house-colors-gw
			"doors": Blue,
			// from house-colors-route-2-rule-1
			"walls": White,
			"floor": Yellow,
		},
		"my-gateway → my-route-2 → my-route-2#rule-2": {
			// from house-colors-gw
			"walls": Black,
			"doors": Blue,
		},
	}

	if len(effectivePoliciesByPath) != len(expectedPolicyRulesByPath) {
		t.Fatalf("expected %d paths, got %d", len(expectedPolicyRulesByPath), len(effectivePoliciesByPath))
	}

	for path, expectedRules := range expectedPolicyRulesByPath {
		effectivePolicy := effectivePoliciesByPath[path]
		effectiveRules := effectivePolicy.Spec.Proper().Rules
		if len(effectiveRules) != len(expectedRules) {
			t.Fatalf("expected %d rules for path %s, got %d", len(expectedRules), path, len(effectiveRules))
		}
		for id, color := range effectiveRules {
			if color != expectedRules[id] {
				t.Errorf("expected rule %s to have color %s for path %s, got %s", id, expectedRules[id], path, color)
			}
		}
	}
}

func buildPolicy(f ...func(*ColorPolicy)) *ColorPolicy {
	policy := &ColorPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kuadrant.io/v1",
			Kind:       "ColorPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-color-policy",
			Namespace: "my-namespace",
		},
		Spec: ColorSpec{
			TargetRef: gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
				LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
					Kind: "Service",
					Name: "my-service",
				},
			},
			ColorSpecProper: ColorSpecProper{
				Rules: map[string]ColorValue{},
			},
		},
	}
	for _, fn := range f {
		fn(policy)
	}
	return policy
}
