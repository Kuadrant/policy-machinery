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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

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
		controller.WithPolicyKinds(
			schema.GroupKind{Group: kuadrantv1alpha2.SchemeGroupVersion.Group, Kind: "DNSPolicy"},
			schema.GroupKind{Group: kuadrantv1alpha2.SchemeGroupVersion.Group, Kind: "TLSPolicy"},
			schema.GroupKind{Group: kuadrantv1beta3.SchemeGroupVersion.Group, Kind: "AuthPolicy"},
			schema.GroupKind{Group: kuadrantv1beta3.SchemeGroupVersion.Group, Kind: "RateLimitPolicy"},
		),
		controller.WithCallback(reconcile),
	)

	controller.Start()
}

func reconcile(eventType controller.EventType, oldObj, newObj controller.RuntimeObject, topology *machinery.Topology) {
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

		// reconcile Gateway -> HTTPRouteRule policies
		for _, httpRouteRule := range httpRouteRules {
			paths := targetables.Paths(gateway, httpRouteRule)
			for i := range paths {
				effectivePolicyForPath[*kuadrantv1beta3.AuthPolicy](paths[i])
				effectivePolicyForPath[*kuadrantv1beta3.RateLimitPolicy](paths[i])
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
