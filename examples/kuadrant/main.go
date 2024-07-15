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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
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

var _ controller.RuntimeObject = &gwapiv1.Gateway{}
var _ controller.RuntimeObject = &gwapiv1.HTTPRoute{}
var _ controller.RuntimeObject = &kuadrantv1alpha2.DNSPolicy{}
var _ controller.RuntimeObject = &kuadrantv1beta3.AuthPolicy{}
var _ controller.RuntimeObject = &kuadrantv1beta3.RateLimitPolicy{}

var securityPolicyKind = schema.GroupKind{Group: egv1alpha1.GroupName, Kind: "SecurityPolicy"}

func main() {
	// load kubeconfig
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	config, err := kubeconfig.ClientConfig()
	if err != nil {
		log.Fatalf("Error loading kubeconfig: %v", err)
	}

	// create the client
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

	controller := controller.NewController(
		controller.WithClient(client),
		controller.WithInformer("gateway", controller.For[*gwapiv1.Gateway](gwapiv1.SchemeGroupVersion.WithResource("gateways"), metav1.NamespaceAll)),
		controller.WithInformer("httproute", controller.For[*gwapiv1.HTTPRoute](gwapiv1.SchemeGroupVersion.WithResource("httproutes"), metav1.NamespaceAll)),
		controller.WithInformer("dnspolicy", controller.For[*kuadrantv1alpha2.DNSPolicy](kuadrantv1alpha2.SchemeGroupVersion.WithResource("dnspolicies"), metav1.NamespaceAll)),
		controller.WithInformer("tlspolicy", controller.For[*kuadrantv1alpha2.TLSPolicy](kuadrantv1alpha2.SchemeGroupVersion.WithResource("tlspolicies"), metav1.NamespaceAll)),
		controller.WithInformer("authpolicy", controller.For[*kuadrantv1beta3.AuthPolicy](kuadrantv1beta3.SchemeGroupVersion.WithResource("authpolicies"), metav1.NamespaceAll)),
		controller.WithInformer("ratelimitpolicy", controller.For[*kuadrantv1beta3.RateLimitPolicy](kuadrantv1beta3.SchemeGroupVersion.WithResource("ratelimitpolicies"), metav1.NamespaceAll)),
		controller.WithInformer("securitypolicy", controller.For[*egv1alpha1.SecurityPolicy](egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies"), metav1.NamespaceAll)),
		controller.WithPolicyKinds(
			schema.GroupKind{Group: kuadrantv1alpha2.SchemeGroupVersion.Group, Kind: "DNSPolicy"},
			schema.GroupKind{Group: kuadrantv1alpha2.SchemeGroupVersion.Group, Kind: "TLSPolicy"},
			schema.GroupKind{Group: kuadrantv1beta3.SchemeGroupVersion.Group, Kind: "AuthPolicy"},
			schema.GroupKind{Group: kuadrantv1beta3.SchemeGroupVersion.Group, Kind: "RateLimitPolicy"},
		),
		controller.WithObjectKinds(securityPolicyKind),
		controller.WithObjectLinks(linkGatewayToSecurityPolicyFunc),
		controller.WithCallback(reconcile(client)),
	)

	controller.Start()
}

func reconcile(client *dynamic.DynamicClient) controller.CallbackFunc {
	return func(eventType controller.EventType, oldObj, newObj controller.RuntimeObject, topology *machinery.Topology) {
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

		for _, gateway := range gateways {
			// reconcile Gateway -> Listener policies
			for _, listener := range listeners {
				paths := targetables.Paths(gateway, listener)
				for i := range paths {
					effectivePolicyForPath[*kuadrantv1alpha2.DNSPolicy](paths[i])
					effectivePolicyForPath[*kuadrantv1alpha2.TLSPolicy](paths[i])
				}
			}

			var hasAuthPolicy bool

			// reconcile Gateway -> HTTPRouteRule policies
			for _, httpRouteRule := range httpRouteRules {
				paths := targetables.Paths(gateway, httpRouteRule)
				for i := range paths {
					if effectivePolicyForPath[*kuadrantv1beta3.AuthPolicy](paths[i]) != nil {
						hasAuthPolicy = true
					}
					effectivePolicyForPath[*kuadrantv1beta3.RateLimitPolicy](paths[i])
				}
			}

			if hasAuthPolicy {
				createSecurityPolicy(client, topology, gateway)
			} else {
				deleteSecurityPolicy(client, topology, gateway)
			}
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

func linkGatewayToSecurityPolicyFunc(objs controller.Store) machinery.LinkFunc {
	gatewayKind := schema.GroupKind{Group: gwapiv1.GroupName, Kind: "Gateway"}
	gateways := lo.FilterMap(lo.Values(objs[gatewayKind]), func(obj controller.RuntimeObject, _ int) (*gwapiv1.Gateway, bool) {
		g, ok := obj.(*gwapiv1.Gateway)
		if !ok {
			return nil, false
		}
		return g, true
	})

	return machinery.LinkFunc{
		From: gatewayKind,
		To:   securityPolicyKind,
		Func: func(child machinery.Object) []machinery.Object {
			o := child.(*controller.Object)
			sp := o.RuntimeObject.(*egv1alpha1.SecurityPolicy)
			refs := sp.Spec.PolicyTargetReferences.TargetRefs
			if ref := sp.Spec.PolicyTargetReferences.TargetRef; ref != nil {
				refs = append(refs, *ref)
			}
			refs = lo.Filter(refs, func(ref gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName, _ int) bool {
				return ref.Group == gwapiv1.GroupName && ref.Kind == gwapiv1.Kind(gatewayKind.Kind)
			})
			if len(refs) == 0 {
				return nil
			}
			gateway, ok := lo.Find(gateways, func(g *gwapiv1.Gateway) bool {
				if g.GetNamespace() != sp.GetNamespace() {
					return false
				}
				return lo.ContainsBy(refs, func(ref gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName) bool {
					return ref.Name == gwapiv1.ObjectName(g.GetName())
				})
			})
			if ok {
				return []machinery.Object{&machinery.Gateway{Gateway: gateway}}
			}
			return nil
		},
	}
}

func createSecurityPolicy(client *dynamic.DynamicClient, topology *machinery.Topology, gateway machinery.Targetable) {
	resource := client.Resource(egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies")).Namespace(gateway.GetNamespace())

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

func deleteSecurityPolicy(client *dynamic.DynamicClient, topology *machinery.Topology, gateway machinery.Targetable) {
	resource := client.Resource(egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies")).Namespace(gateway.GetNamespace())

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
