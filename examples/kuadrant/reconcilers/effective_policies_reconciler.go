package reconcilers

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"sync"

	"github.com/samber/lo"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantapis "github.com/kuadrant/policy-machinery/examples/kuadrant/apis"
	kuadrantv1alpha2 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1alpha2"
	kuadrantv1beta3 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1beta3"
)

const authPathsKey = "authPaths"

func ReconcileEffectivePolicies(ctx context.Context, resourceEvents []controller.ResourceEvent, topology *machinery.Topology, err error, state *sync.Map) {
	targetables := topology.Targetables()

	// reconcile policies
	gateways := targetables.Items(func(o machinery.Object) bool {
		_, ok := o.(*machinery.Gateway)
		return ok
	})

	listeners := targetables.Items(func(o machinery.Object) bool {
		_, ok := o.(*machinery.Listener)
		return ok
	})

	httpRouteRules := targetables.Items(func(o machinery.Object) bool {
		_, ok := o.(*machinery.HTTPRouteRule)
		return ok
	})

	var authPaths [][]machinery.Targetable

	for _, gateway := range gateways {
		// reconcile Gateway -> Listener policies
		for _, listener := range listeners {
			paths := targetables.Paths(gateway, listener)
			for i := range paths {
				if p := effectivePolicyForPath[*kuadrantv1alpha2.DNSPolicy](ctx, paths[i]); p != nil {
					// TODO: reconcile dns effective policy (i.e. create the DNSRecords for it)
				}
				if p := effectivePolicyForPath[*kuadrantv1alpha2.TLSPolicy](ctx, paths[i]); p != nil {
					// TODO: reconcile tls effective policy (i.e. create the certificate request for it)
				}
			}
		}

		// reconcile Gateway -> HTTPRouteRule policies
		for _, httpRouteRule := range httpRouteRules {
			paths := targetables.Paths(gateway, httpRouteRule)
			for i := range paths {
				if p := effectivePolicyForPath[*kuadrantv1beta3.AuthPolicy](ctx, paths[i]); p != nil {
					authPaths = append(authPaths, paths[i])
					// TODO: reconcile auth effective policy (i.e. create the Authorino AuthConfig)
				}
				if p := effectivePolicyForPath[*kuadrantv1beta3.RateLimitPolicy](ctx, paths[i]); p != nil {
					// TODO: reconcile rate-limit effective policy (i.e. create the Limitador limits config)
				}
			}
		}
	}

	state.Store(authPathsKey, authPaths)
}

func effectivePolicyForPath[T machinery.Policy](ctx context.Context, path []machinery.Targetable) *T {
	logger := controller.LoggerFromContext(ctx).WithName("effective policy")

	// gather all policies in the path sorted from the least specific to the most specific
	policies := lo.FlatMap(path, func(targetable machinery.Targetable, _ int) []machinery.Policy {
		policies := lo.FilterMap(targetable.Policies(), func(p machinery.Policy, _ int) (kuadrantapis.MergeablePolicy, bool) {
			_, ok := p.(T)
			mergeablePolicy, mergeable := p.(kuadrantapis.MergeablePolicy)
			return mergeablePolicy, mergeable && ok
		})
		sort.Sort(kuadrantapis.PolicyByCreationTimestamp(policies))
		return lo.Map(policies, func(p kuadrantapis.MergeablePolicy, _ int) machinery.Policy { return p })
	})

	pathLocators := lo.Map(path, machinery.MapTargetableToLocatorFunc)

	if len(policies) == 0 {
		logger.Info("no policies for path", "kind", reflect.TypeOf(new(T)), "path", pathLocators)
		return nil
	}

	// map reduces the policies from most specific to least specific, merging them into one effective policy
	effectivePolicy := lo.ReduceRight(policies, func(effectivePolicy machinery.Policy, policy machinery.Policy, _ int) machinery.Policy {
		return effectivePolicy.Merge(policy)
	}, policies[len(policies)-1])

	jsonEffectivePolicy, _ := json.Marshal(effectivePolicy)
	logger.Info("effective policy", "kind", reflect.TypeOf(new(T)), "path", pathLocators, "effectivePolicy", string(jsonEffectivePolicy))

	concreteEffectivePolicy, _ := effectivePolicy.(T)
	return &concreteEffectivePolicy
}
