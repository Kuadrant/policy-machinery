package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/samber/lo"
	"k8s.io/client-go/dynamic"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantapis "github.com/kuadrant/policy-machinery/examples/kuadrant/apis"
	kuadrantv1alpha2 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1alpha2"
	kuadrantv1beta3 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1beta3"
)

const authPathsKey = "authPaths"

// EffectivePoliciesReconciler works exactly like a controller.Workflow where the precondition reconcile function
// reconciles the effective policies for the given topology paths, occasionally modifying the context that is passed
// as argument to the subsequent concurrent reconcilers.
type EffectivePoliciesReconciler struct {
	Client         *dynamic.DynamicClient
	ReconcileFuncs []controller.CallbackFunc
}

func (r *EffectivePoliciesReconciler) Reconcile(ctx context.Context, resourceEvent controller.ResourceEvent, topology *machinery.Topology) {
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

	for _, gateway := range gateways {
		// reconcile Gateway -> Listener policies
		for _, listener := range listeners {
			paths := targetables.Paths(gateway, listener)
			for i := range paths {
				if p := effectivePolicyForPath[*kuadrantv1alpha2.DNSPolicy](paths[i]); p != nil {
					// TODO: reconcile dns effective policy (i.e. create the DNSRecords for it)
				}
				if p := effectivePolicyForPath[*kuadrantv1alpha2.TLSPolicy](paths[i]); p != nil {
					// TODO: reconcile tls effective policy (i.e. create the certificate request for it)
				}
			}
		}

		// reconcile Gateway -> HTTPRouteRule policies
		for _, httpRouteRule := range httpRouteRules {
			paths := targetables.Paths(gateway, httpRouteRule)
			for i := range paths {
				if p := effectivePolicyForPath[*kuadrantv1beta3.AuthPolicy](paths[i]); p != nil {
					ctx = pathIntoContext(ctx, authPathsKey, paths[i])
					// TODO: reconcile auth effective policy (i.e. create the Authorino AuthConfig)
				}
				if p := effectivePolicyForPath[*kuadrantv1beta3.RateLimitPolicy](paths[i]); p != nil {
					// TODO: reconcile rate-limit effective policy (i.e. create the Limitador limits config)
				}
			}
		}
	}

	// dispatch the event to subsequent reconcilers
	funcs := r.ReconcileFuncs
	waitGroup := &sync.WaitGroup{}
	defer waitGroup.Wait()
	waitGroup.Add(len(funcs))
	for _, f := range funcs {
		go func() {
			defer waitGroup.Done()
			f(ctx, resourceEvent, topology)
		}()
	}
}

func effectivePolicyForPath[T machinery.Policy](path []machinery.Targetable) *T {
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

	pathStr := strings.Join(lo.Map(path, func(t machinery.Targetable, _ int) string {
		return fmt.Sprintf("%s::%s/%s", t.GroupVersionKind().Kind, t.GetNamespace(), t.GetName())
	}), " → ")

	if len(policies) == 0 {
		log.Printf("No %T for path %s\n", new(T), pathStr)
		return nil
	}

	// map reduces the policies from most specific to least specific, merging them into one effective policy
	effectivePolicy := lo.ReduceRight(policies, func(effectivePolicy machinery.Policy, policy machinery.Policy, _ int) machinery.Policy {
		return effectivePolicy.Merge(policy)
	}, policies[len(policies)-1])

	jsonEffectivePolicy, _ := json.MarshalIndent(effectivePolicy, "", "  ")
	log.Printf("Effective %T for path %s:\n%s\n", new(T), pathStr, jsonEffectivePolicy)

	concreteEffectivePolicy, _ := effectivePolicy.(T)
	return &concreteEffectivePolicy
}

func pathIntoContext(ctx context.Context, key string, path []machinery.Targetable) context.Context {
	if p := ctx.Value(key); p != nil {
		return context.WithValue(ctx, key, append(p.([][]machinery.Targetable), path))
	}
	return context.WithValue(ctx, key, [][]machinery.Targetable{path})
}

func pathsFromContext(ctx context.Context, key string) [][]machinery.Targetable {
	var paths [][]machinery.Targetable
	if p := ctx.Value(key); p != nil {
		paths = p.([][]machinery.Targetable)
	}
	return paths
}
