package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantapis "github.com/kuadrant/policy-machinery/examples/kuadrant/apis"
	kuadrantv1alpha2 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1alpha2"
	kuadrantv1beta3 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1beta3"
)

const topologyFile = "topology.dot"

type GatewayProvider interface {
	ReconcileGateway(topology *machinery.Topology, gateway machinery.Targetable, capabilities map[string][][]machinery.Targetable)
}

type Reconciler struct {
	GatewayProviders []GatewayProvider
}

func (r *Reconciler) Reconcile(eventType controller.EventType, oldObj, newObj controller.RuntimeObject, topology *machinery.Topology) {
	// print the event
	obj := oldObj
	if obj == nil {
		obj = newObj
	}
	log.Printf("%s %sd: %s/%s\n", obj.GetObjectKind().GroupVersionKind().Kind, eventType.String(), obj.GetNamespace(), obj.GetName())
	if eventType == controller.UpdateEvent {
		log.Println(cmp.Diff(oldObj, newObj))
	}

	// update the topology file
	saveTopologyToFile(topology)

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

	capabilities := map[string][][]machinery.Targetable{
		"auth":      {},
		"ratelimit": {},
	}

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
					capabilities["auth"] = append(capabilities["auth"], paths[i])
					// TODO: reconcile auth effective policy (i.e. create the Authorino AuthConfig)
				}
				if p := effectivePolicyForPath[*kuadrantv1beta3.RateLimitPolicy](paths[i]); p != nil {
					capabilities["ratelimit"] = append(capabilities["ratelimit"], paths[i])
					// TODO: reconcile rate-limit effective policy (i.e. create the Limitador limits config)
				}
			}
		}

		for _, gatewayProvider := range r.GatewayProviders {
			gatewayProvider.ReconcileGateway(topology, gateway, capabilities)
		}
	}
}

func saveTopologyToFile(topology *machinery.Topology) {
	file, err := os.Create(topologyFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	_, err = file.Write(topology.ToDot().Bytes())
	if err != nil {
		log.Fatal(err)
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

var _ GatewayProvider = &DefaultGatewayProvider{}

type DefaultGatewayProvider struct{}

func (p *DefaultGatewayProvider) ReconcileGateway(_ *machinery.Topology, _ machinery.Targetable, _ map[string][][]machinery.Targetable) {
}
