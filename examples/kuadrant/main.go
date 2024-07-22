package main

import (
	"log"
	"os"
	"strings"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/samber/lo"
	istiov1 "istio.io/client-go/pkg/apis/security/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/policy-machinery/controller"

	kuadrantv1alpha2 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1alpha2"
	kuadrantv1beta3 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1beta3"
)

var supportedGatewayProviders = []string{envoyGatewayProvider, istioGatewayProvider}

func main() {
	var gatewayProviders []string
	for i := range os.Args {
		switch os.Args[i] {
		case "--gateway-providers":
			{
				defer func() {
					if recover() != nil {
						log.Fatalf("Invalid gateway provider. Supported: %s\n", strings.Join(supportedGatewayProviders, ","))
					}
				}()
				gatewayProviders = lo.Map(strings.Split(os.Args[i+1], ","), func(gp string, _ int) string {
					return strings.TrimSpace(gp)
				})
				if !lo.Every(supportedGatewayProviders, gatewayProviders) {
					panic("")
				}
			}
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
		controller.WithCallback(buildReconciler(gatewayProviders, client).Reconcile),
	}
	controllerOpts = append(controllerOpts, controllerOptionsFor(gatewayProviders)...)

	controller.NewController(controllerOpts...).Start()
}

func buildReconciler(gatewayProviders []string, client *dynamic.DynamicClient) *Reconciler {
	var providers []GatewayProvider

	for _, gatewayProvider := range gatewayProviders {
		switch gatewayProvider {
		case envoyGatewayProvider:
			providers = append(providers, &EnvoyGatewayProvider{client})
		case istioGatewayProvider:
			providers = append(providers, &IstioGatewayProvider{client})
		}
	}

	if len(providers) == 0 {
		providers = append(providers, &DefaultGatewayProvider{})
	}

	return &Reconciler{
		GatewayProviders: providers,
	}
}

func controllerOptionsFor(gatewayProviders []string) []controller.ControllerOptionFunc {
	var opts []controller.ControllerOptionFunc

	// if we care about specificities of gateway controllers, then let's add gateway classes to the topology too
	if len(gatewayProviders) > 0 {
		opts = append(opts, controller.WithInformer("gatewayclass", controller.For[*gwapiv1.GatewayClass](gwapiv1.SchemeGroupVersion.WithResource("gatewayclasses"), metav1.NamespaceNone)))
	}

	for _, gatewayProvider := range gatewayProviders {
		switch gatewayProvider {
		case envoyGatewayProvider:
			opts = append(opts, controller.WithInformer("envoygateway/securitypolicy", controller.For[*egv1alpha1.SecurityPolicy](envoyGatewaySecurityPoliciesResource, metav1.NamespaceAll)))
			opts = append(opts, controller.WithObjectKinds(envoyGatewaySecurityPolicyKind))
			opts = append(opts, controller.WithObjectLinks(linkGatewayToEnvoyGatewaySecurityPolicyFunc))
		case istioGatewayProvider:
			opts = append(opts, controller.WithInformer("istio/authorizationpolicy", controller.For[*istiov1.AuthorizationPolicy](istioAuthorizationPoliciesResource, metav1.NamespaceAll)))
			opts = append(opts, controller.WithObjectKinds(istioAuthorizationPolicyKind))
			opts = append(opts, controller.WithObjectLinks(linkGatewayToIstioAuthorizationPolicyFunc))
		}
	}

	return opts
}
