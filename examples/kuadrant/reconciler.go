package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantapis "github.com/kuadrant/policy-machinery/examples/kuadrant/apis"
	kuadrantv1alpha2 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1alpha2"
	kuadrantv1beta3 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1beta3"
)

const topologyFile = "topology.dot"

type GatewayProvider interface {
	ReconcileGateway(topology *machinery.Topology, gateway machinery.Targetable, effectivePolicies map[string][]machinery.Policy)
}

type Reconciler struct {
	GatewayProvider GatewayProvider
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

	effectivePolicies := map[string][]machinery.Policy{}

	for _, gateway := range gateways {
		// reconcile Gateway -> Listener policies
		for _, listener := range listeners {
			paths := targetables.Paths(gateway, listener)
			for i := range paths {
				if p := effectivePolicyForPath[*kuadrantv1alpha2.DNSPolicy](paths[i]); p != nil {
					effectivePolicies["dns"] = append(effectivePolicies["dns"], *p)
				}
				if p := effectivePolicyForPath[*kuadrantv1alpha2.TLSPolicy](paths[i]); p != nil {
					effectivePolicies["tls"] = append(effectivePolicies["tls"], *p)
				}
			}
		}

		// reconcile Gateway -> HTTPRouteRule policies
		for _, httpRouteRule := range httpRouteRules {
			paths := targetables.Paths(gateway, httpRouteRule)
			for i := range paths {
				if p := effectivePolicyForPath[*kuadrantv1beta3.AuthPolicy](paths[i]); p != nil {
					effectivePolicies["auth"] = append(effectivePolicies["auth"], *p)
				}
				if p := effectivePolicyForPath[*kuadrantv1beta3.RateLimitPolicy](paths[i]); p != nil {
					effectivePolicies["ratelimit"] = append(effectivePolicies["ratelimit"], *p)
				}
			}
		}

		r.GatewayProvider.ReconcileGateway(topology, gateway, effectivePolicies)
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

func (p *DefaultGatewayProvider) ReconcileGateway(_ *machinery.Topology, _ machinery.Targetable, _ map[string][]machinery.Policy) {
}

var _ GatewayProvider = &EnvoyGatewayProvider{}

type EnvoyGatewayProvider struct {
	*dynamic.DynamicClient
}

func (p *EnvoyGatewayProvider) ReconcileGateway(topology *machinery.Topology, gateway machinery.Targetable, effectivePolicies map[string][]machinery.Policy) {
	// check if the gateway is managed by the envoy gateway controller
	if !lo.ContainsBy(topology.Targetables().Parents(gateway), func(p machinery.Targetable) bool {
		gc, ok := p.(*machinery.GatewayClass)
		return ok && gc.Spec.ControllerName == "gateway.envoyproxy.io/gatewayclass-controller"
	}) {
		return
	}

	// reconcile envoy gateway securitypolicy resources
	if len(effectivePolicies["auth"]) > 0 {
		p.createSecurityPolicy(topology, gateway)
		return
	}
	p.deleteSecurityPolicy(topology, gateway)
}

func (p *EnvoyGatewayProvider) createSecurityPolicy(topology *machinery.Topology, gateway machinery.Targetable) {
	resource := p.Resource(egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies")).Namespace(gateway.GetNamespace())

	obj, found := lo.Find(topology.Objects().Children(gateway), func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == securityPolicyKind && o.GetNamespace() == gateway.GetNamespace() && o.GetName() == gateway.GetName()
	})

	desiredSecurityPolicy := &egv1alpha1.SecurityPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: egv1alpha1.GroupVersion.String(),
			Kind:       securityPolicyKind.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.GetName(),
			Namespace: gateway.GetNamespace(),
		},
		Spec: egv1alpha1.SecurityPolicySpec{
			PolicyTargetReferences: egv1alpha1.PolicyTargetReferences{
				TargetRef: &gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
					LocalPolicyTargetReference: gwapiv1alpha2.LocalPolicyTargetReference{
						Group: gwapiv1alpha2.GroupName,
						Kind:  gwapiv1alpha2.Kind("Gateway"),
						Name:  gwapiv1.ObjectName(gateway.GetName()),
					},
				},
			},
			ExtAuth: &egv1alpha1.ExtAuth{
				GRPC: &egv1alpha1.GRPCExtAuthService{
					BackendRef: &gwapiv1.BackendObjectReference{
						Name:      gwapiv1.ObjectName("authorino-authorino-authorization"),
						Namespace: ptr.To(gwapiv1.Namespace("kuadrant-system")),
						Port:      ptr.To(gwapiv1.PortNumber(50051)),
					},
				},
			},
		},
	}

	if !found {
		o, _ := controller.Destruct(desiredSecurityPolicy)
		_, err := resource.Create(context.TODO(), o, metav1.CreateOptions{})
		if err != nil {
			log.Println("failed to create SecurityPolicy", err)
		}
		return
	}

	securityPolicy := obj.(*controller.Object).RuntimeObject.(*egv1alpha1.SecurityPolicy)

	if securityPolicy.Spec.ExtAuth == nil ||
		securityPolicy.Spec.ExtAuth.GRPC == nil ||
		securityPolicy.Spec.ExtAuth.GRPC.BackendRef == nil ||
		securityPolicy.Spec.ExtAuth.GRPC.BackendRef.Namespace != desiredSecurityPolicy.Spec.ExtAuth.GRPC.BackendRef.Namespace ||
		securityPolicy.Spec.ExtAuth.GRPC.BackendRef.Name != desiredSecurityPolicy.Spec.ExtAuth.GRPC.BackendRef.Name ||
		securityPolicy.Spec.ExtAuth.GRPC.BackendRef.Port != desiredSecurityPolicy.Spec.ExtAuth.GRPC.BackendRef.Port {
		return
	}

	securityPolicy.Spec = desiredSecurityPolicy.Spec
	o, _ := controller.Destruct(securityPolicy)
	_, err := resource.Update(context.TODO(), o, metav1.UpdateOptions{})
	if err != nil {
		log.Println("failed to update SecurityPolicy", err)
	}
}

func (p *EnvoyGatewayProvider) deleteSecurityPolicy(topology *machinery.Topology, gateway machinery.Targetable) {
	resource := p.Resource(egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies")).Namespace(gateway.GetNamespace())

	_, found := lo.Find(topology.Objects().Children(gateway), func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == securityPolicyKind && o.GetNamespace() == gateway.GetNamespace() && o.GetName() == gateway.GetName()
	})

	if !found {
		return
	}

	err := resource.Delete(context.TODO(), gateway.GetName(), metav1.DeleteOptions{})
	if err != nil {
		log.Println("failed to delete SecurityPolicy", err)
	}
}
