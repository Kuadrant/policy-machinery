package main

import (
	"log"
	"os"
	"strings"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantv1alpha2 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1alpha2"
	kuadrantv1beta3 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1beta3"
)

const envoyGatewayProvider = "envoygateway"

var (
	supportedGatewayProviders = []string{envoyGatewayProvider}

	securityPolicyKind = schema.GroupKind{Group: egv1alpha1.GroupName, Kind: "SecurityPolicy"}
)

func main() {
	var gatewayProvider string
	for i := range os.Args {
		switch os.Args[i] {
		case "--gateway-provider":
			if i == len(os.Args)-1 || !lo.Contains(supportedGatewayProviders, os.Args[i+1]) {
				log.Fatalf("Invalid gateway provider. Use one of: %s\n", strings.Join(supportedGatewayProviders, ","))
			}
			gatewayProvider = os.Args[i+1]
		}
	}

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

	controllerOpts := []controller.ControllerOptionFunc{
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
		controller.WithCallback(buildReconcilerFor(gatewayProvider, client).Reconcile),
	}
	controllerOpts = append(controllerOpts, controllerOptionsFor(gatewayProvider)...)

	controller.NewController(controllerOpts...).Start()
}

func buildReconcilerFor(gatewayProvider string, client *dynamic.DynamicClient) *Reconciler {
	var provider GatewayProvider

	switch gatewayProvider {
	case envoyGatewayProvider:
		provider = &EnvoyGatewayProvider{client}
	default:
		provider = &DefaultGatewayProvider{}
	}

	return &Reconciler{
		GatewayProvider: provider,
	}
}

func controllerOptionsFor(gatewayProvider string) []controller.ControllerOptionFunc {
	switch gatewayProvider {
	case envoyGatewayProvider:
		return []controller.ControllerOptionFunc{
			controller.WithInformer("gatewayclass", controller.For[*gwapiv1.GatewayClass](gwapiv1.SchemeGroupVersion.WithResource("gatewayclasses"), metav1.NamespaceNone)),
			controller.WithInformer("securitypolicy", controller.For[*egv1alpha1.SecurityPolicy](egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies"), metav1.NamespaceAll)),
			controller.WithObjectKinds(securityPolicyKind),
			controller.WithObjectLinks(linkGatewayToSecurityPolicyFunc),
		}
	default:
		return nil
	}
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
