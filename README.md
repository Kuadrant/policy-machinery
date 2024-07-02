# Policy Machinery

Machinery for implementing [Gateway API](https://gateway-api.sigs.k8s.io/reference/policy-attachment/) policies.

## Features
- `Topology` struct for modeling topologies of targetable network resources and corresponding attached policies
- Examples of policy struct (`ColorPolicy`) and implemented merge strategies based on Kuadrant's Defaults & Overrides
  ([RFC 0009](https://docs.kuadrant.io/0.8.0/architecture/rfcs/0009-defaults-and-overrides/)): atomic defaults, atomic
  overrides, merge policy rule defaults, merge policy rule overrides
- Helpers for testing your own topologies of Gateway API resources and policies

## Use

① Import the package:

```sh
go get github.com/kuadrant/policy-machinery
```

② Implement the `Policy` interface:

```go
package mypolicy

import (
  "github.com/kuadrant/policy-machinery/machinery"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

var _ machinery.Policy = &MyPolicy{}

type MyPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MyPolicySpec `json:"spec"`
}

type MyPolicySpec struct {
  TargetRef gwapiv1alpha2.LocalPolicyTargetReference
}

func (p *MyPolicy) GetURL() string {
	return machinery.UrlFromObject(p)
}

func (p *MyPolicy) GetTargetRefs() []machinery.PolicyTargetReference {
	return []machinery.PolicyTargetReference{
		machinery.LocalPolicyTargetReference{
			LocalPolicyTargetReference: p.Spec.TargetRef,
			PolicyNamespace: p.Namespace,
		},
	}
}

func (p *MyPolicy) GetMergeStrategy() machinery.MergeStrategy {
	return machinery.DefaultMergeStrategy // replace with your merge strategy
}

func (p *MyPolicy) Merge(policy machinery.Policy) machinery.Policy {
	source := policy.(*MyPolicy)
	return source.GetMergeStrategy()(source, p)
}

func (p *MyPolicy) DeepCopy() *MyPolicy {
	spec := p.Spec.DeepCopy()
	return &MyPolicy{
		TypeMeta:   p.TypeMeta,
		ObjectMeta: p.ObjectMeta,
		Spec:       *spec,
	}
}
```

③ Build a topology of targetable network resources:

```go
import (
  "fmt"

  "github.com/kuadrant/policy-machinery/machinery"
  "github.com/samber/lo"
)

// ...

topology := machinery.NewTopology(
  machinery.WithTargetables(gateways...),
  machinery.WithTargetables(httpRoutes...),
  machinery.WithTargetables(services...),
  machinery.WithLinks(
    machinery.LinkGatewayToHTTPRouteFunc(gateways),
    machinery.LinkHTTPRouteToServiceFunc(httpRoutes, false),
  ),
  machinery.WithPolicies(policies...),
)

// Print the topology in Graphviz DOT language
fmt.Println(topology.ToDot())

// Calculate the effective policy for any path between 2 targetables in the topology
paths := topology.Paths(gateways[0], services[0])

for _, path := range paths {
  // Gather all policies in the path sorted from the least specific to the most specific
  policies := lo.FlatMap(path, func(targetable machinery.Targetable, _ int) []machinery.Policy {
    return targetable.Policies()
  })

  // Map reduces the policies from most specific to least specific, merging them into one effective policy for each path
  var emptyPolicy machinery.Policy = &MyPolicy{}
  effectivePolicy := lo.ReduceRight(policies, func(effectivePolicy machinery.Policy, policy machinery.Policy, _ int) machinery.Policy {
    return effectivePolicy.Merge(policy)
  }, emptyPolicy)
}
```

Alternatively, use the `NewGatewayAPITopology` helper function to build a topology of Gateway API resources.
The links between objects will be inferred automatically according to the specs. I.e.:

```go
topology := machinery.NewGatewayAPITopology(
  machinery.WithGateways(gateways...),
  machinery.WithHTTPRoutes(httpRoutes...),
  machinery.WithServices(services...),
  machinery.WithPolicies(policies...),
)
```

> **Tip:** You can use the topology option functions `ExpandGatewayListeners()`, `ExpandHTTPRouteRules()`,
> `ExpandServicePorts()` to automatically expand Gateways, HTTPRoutes and Services so their inner sections
> (listeners, route rules, service ports) are added as targetables to the topology. The links between objects
> are then adjusted accordingly.