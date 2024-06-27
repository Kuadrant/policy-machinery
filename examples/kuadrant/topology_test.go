//go:build integration

package kuadrant

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	machinery "github.com/guicassolato/policy-machinery/machinery"
)

// TestMergeBasedOnTopology tests ColorPolicy's merge strategies for painting a house, based on network traffic
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
func TestMergeBasedOnTopology(t *testing.T) {
	gateways := []*machinery.Gateway{{Gateway: machinery.BuildGateway()}}
	httpRoutes := []*machinery.HTTPRoute{
		{HTTPRoute: machinery.BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
			r.Name = "my-route-1"
			r.Spec.Rules = append(r.Spec.Rules, gwapiv1.HTTPRouteRule{
				BackendRefs: []gwapiv1.HTTPBackendRef{machinery.BuildHTTPBackendRef()},
			})
		})},
		{HTTPRoute: machinery.BuildHTTPRoute(func(r *gwapiv1.HTTPRoute) {
			r.Name = "my-route-2"
			r.Spec.Rules = append(r.Spec.Rules, gwapiv1.HTTPRouteRule{
				BackendRefs: []gwapiv1.HTTPBackendRef{machinery.BuildHTTPBackendRef()},
			})
		})},
	}
	httpRouteRules := lo.FlatMap(httpRoutes, machinery.HTTPRouteRulesFromHTTPRouteFunc)
	services := []*machinery.Service{{Service: machinery.BuildService()}}
	policies := []machinery.Policy{
		buildPolicy(func(p *ColorPolicy) { // atomic defaults
			p.Name = "house-colors-gw"
			p.Spec.TargetRef.Group = gwapiv1.GroupName
			p.Spec.TargetRef.Kind = "Gateway"
			p.Spec.TargetRef.Name = "my-gateway"
			p.Spec.ColorSpecProper = ColorSpecProper{}
			p.Spec.Defaults = &MergeableColorSpec{
				Strategy: AtomicMergeStrategy,
				ColorSpecProper: ColorSpecProper{
					Rules: []ColorRule{
						{
							Id:    "walls",
							Color: Black,
						},
						{
							Id:    "doors",
							Color: Blue,
						},
					},
				},
			}
		}),
		buildPolicy(func(p *ColorPolicy) { // policy rule overrides
			p.Name = "house-colors-route-1"
			p.Spec.TargetRef.Group = gwapiv1.GroupName
			p.Spec.TargetRef.Kind = "HTTPRoute"
			p.Spec.TargetRef.Name = "my-route-1"
			p.Spec.ColorSpecProper = ColorSpecProper{}
			p.Spec.Overrides = &MergeableColorSpec{
				Strategy: PolicyRuleMergeStrategy,
				ColorSpecProper: ColorSpecProper{
					Rules: []ColorRule{
						{
							Id:    "walls",
							Color: Green,
						},
						{
							Id:    "roof",
							Color: Orange,
						},
					},
				},
			}
		}),
		buildPolicy(func(p *ColorPolicy) { // default: atomic defaults
			p.Name = "house-colors-route-1-rule-1"
			p.Spec.TargetRef.Group = gwapiv1.GroupName
			p.Spec.TargetRef.Kind = "HTTPRoute"
			p.Spec.TargetRef.Name = "my-route-1"
			p.Spec.TargetRef.SectionName = ptr.To(gwapiv1.SectionName("rule-1"))
			p.Spec.Rules = []ColorRule{
				{
					Id:    "roof",
					Color: Purple,
				},
				{
					Id:    "floor",
					Color: Red,
				},
			}
		}),
		buildPolicy(func(p *ColorPolicy) { // default: atomic defaults
			p.Name = "house-colors-route-2-rule-1"
			p.Spec.TargetRef.Group = gwapiv1.GroupName
			p.Spec.TargetRef.Kind = "HTTPRoute"
			p.Spec.TargetRef.Name = "my-route-2"
			p.Spec.TargetRef.SectionName = ptr.To(gwapiv1.SectionName("rule-1"))
			p.Spec.Rules = []ColorRule{
				{
					Id:    "walls",
					Color: White,
				},
				{
					Id:    "floor",
					Color: Yellow,
				},
			}
		}),
	}

	topology := machinery.NewTopology(
		machinery.WithTargetables(gateways...),
		machinery.WithTargetables(httpRoutes...),
		machinery.WithTargetables(httpRouteRules...),
		machinery.WithTargetables(services...),
		machinery.WithLinks(
			machinery.LinkGatewayToHTTPRouteFunc(gateways),
			machinery.LinkHTTPRouteToHTTPRouteRuleFunc(),
			machinery.LinkHTTPRouteRuleToServiceFunc(httpRouteRules),
		),
		machinery.WithPolicies(policies...),
	)

	for _, httpRouteRule := range httpRouteRules {
		for _, path := range topology.Paths(gateways[0], httpRouteRule) {
			var emptyPolicy machinery.Policy = buildPolicy()
			policies := lo.FlatMap(path, func(targetable machinery.Targetable, _ int) []machinery.Policy {
				return targetable.Policies()
			})
			effectivePolicy := lo.ReduceRight(policies, func(effectivePolicy machinery.Policy, policy machinery.Policy, _ int) machinery.Policy {
				return effectivePolicy.Merge(policy)
			}, emptyPolicy)
			// TODO(guicassolato): Assert the effective policy
			jsonPolicy, _ := json.MarshalIndent(effectivePolicy, "", "  ")
			fmt.Println("Path:", strings.Join(lo.Map(path, func(t machinery.Targetable, _ int) string { return t.GetName() }), " → "))
			fmt.Println("Num. of policies:", len(policies))
			fmt.Println("Effective policy:", string(jsonPolicy))
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
				Rules: []ColorRule{},
			},
		},
	}
	for _, fn := range f {
		fn(policy)
	}
	return policy
}
