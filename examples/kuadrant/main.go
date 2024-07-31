package main

import (
	"context"
	"log"
	"os"
	"strings"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	istiov1 "istio.io/client-go/pkg/apis/security/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantv1alpha2 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1alpha2"
	kuadrantv1beta3 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1beta3"
	"github.com/kuadrant/policy-machinery/examples/kuadrant/reconcilers"
)

var supportedGatewayProviders = []string{reconcilers.EnvoyGatewayProviderName, reconcilers.IstioGatewayProviderName}

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
		controller.WithRunnable("gateway informer", controller.Watch[*gwapiv1.Gateway](controller.GatewaysResource, metav1.NamespaceAll)),
		controller.WithRunnable("httproute informer", controller.Watch[*gwapiv1.HTTPRoute](controller.HTTPRoutesResource, metav1.NamespaceAll)),
		controller.WithRunnable("dnspolicy informer", controller.Watch[*kuadrantv1alpha2.DNSPolicy](kuadrantv1alpha2.DNSPoliciesResource, metav1.NamespaceAll)),
		controller.WithRunnable("tlspolicy informer", controller.Watch[*kuadrantv1alpha2.TLSPolicy](kuadrantv1alpha2.TLSPoliciesResource, metav1.NamespaceAll)),
		controller.WithRunnable("authpolicy informer", controller.Watch[*kuadrantv1beta3.AuthPolicy](kuadrantv1beta3.AuthPoliciesResource, metav1.NamespaceAll)),
		controller.WithRunnable("ratelimitpolicy informer", controller.Watch[*kuadrantv1beta3.RateLimitPolicy](kuadrantv1beta3.RateLimitPoliciesResource, metav1.NamespaceAll)),
		controller.WithPolicyKinds(
			kuadrantv1alpha2.DNSPolicyKind,
			kuadrantv1alpha2.TLSPolicyKind,
			kuadrantv1beta3.AuthPolicyKind,
			kuadrantv1beta3.RateLimitPolicyKind,
		),
		controller.WithCallback(buildReconciler(gatewayProviders, client)),
	}
	controllerOpts = append(controllerOpts, controllerOptionsFor(gatewayProviders)...)

	controller.NewController(controllerOpts...).Start()
}

// buildReconciler builds a reconciler that executes the following workflow:
//  1. log event
//  2. save topology to file
//  3. effective policies
//  4. (gateway deleted) delete SecurityPolicy / (other events) reconcile SecurityPolicies
//  4. (gateway deleted) delete AuthorizationPolicy / (other events) reconcile AuthorizationPolicies
func buildReconciler(gatewayProviders []string, client *dynamic.DynamicClient) controller.CallbackFunc {
	effectivePolicyReconciler := &reconcilers.EffectivePoliciesReconciler{Client: client}

	commonAuthPolicyResourceEventMatchers := []controller.ResourceEventMatcher{
		{Resource: ptr.To(controller.GatewayClassesResource)},
		{Resource: ptr.To(controller.GatewaysResource), EventType: ptr.To(controller.CreateEvent)},
		{Resource: ptr.To(controller.GatewaysResource), EventType: ptr.To(controller.UpdateEvent)},
		{Resource: ptr.To(controller.HTTPRoutesResource)},
		{Resource: ptr.To(kuadrantv1beta3.AuthPoliciesResource)},
	}

	for _, gatewayProvider := range gatewayProviders {
		switch gatewayProvider {
		case reconcilers.EnvoyGatewayProviderName:
			envoyGatewayProvider := &reconcilers.EnvoyGatewayProvider{Client: client}
			effectivePolicyReconciler.ReconcileFuncs = append(effectivePolicyReconciler.ReconcileFuncs, (&controller.Subscriber{
				{
					ReconcileFunc: envoyGatewayProvider.ReconcileSecurityPolicies,
					Events:        append(commonAuthPolicyResourceEventMatchers, controller.ResourceEventMatcher{Resource: ptr.To(reconcilers.EnvoyGatewaySecurityPoliciesResource)}),
				},
				{
					ReconcileFunc: envoyGatewayProvider.DeleteSecurityPolicy,
					Events: []controller.ResourceEventMatcher{
						{Resource: ptr.To(controller.GatewaysResource), EventType: ptr.To(controller.DeleteEvent)},
					},
				},
			}).Reconcile)
		case reconcilers.IstioGatewayProviderName:
			istioGatewayProvider := &reconcilers.IstioGatewayProvider{Client: client}
			effectivePolicyReconciler.ReconcileFuncs = append(effectivePolicyReconciler.ReconcileFuncs, (&controller.Subscriber{
				{
					ReconcileFunc: istioGatewayProvider.ReconcileAuthorizationPolicies,
					Events:        append(commonAuthPolicyResourceEventMatchers, controller.ResourceEventMatcher{Resource: ptr.To(reconcilers.IstioAuthorizationPoliciesResource)}),
				},
				{
					ReconcileFunc: istioGatewayProvider.DeleteAuthorizationPolicy,
					Events: []controller.ResourceEventMatcher{
						{Resource: ptr.To(controller.GatewaysResource), EventType: ptr.To(controller.DeleteEvent)},
					},
				},
			}).Reconcile)
		}
	}

	reconciler := &controller.Workflow{
		Precondition: func(_ context.Context, resourceEvent controller.ResourceEvent, topology *machinery.Topology) {
			// log the event
			obj := resourceEvent.OldObject
			if obj == nil {
				obj = resourceEvent.NewObject
			}
			log.Printf("%s %sd: %s/%s\n", obj.GetObjectKind().GroupVersionKind().Kind, resourceEvent.EventType.String(), obj.GetNamespace(), obj.GetName())
			if resourceEvent.EventType == controller.UpdateEvent {
				log.Println(cmp.Diff(resourceEvent.OldObject, resourceEvent.NewObject))
			}
		},
		Tasks: []controller.CallbackFunc{
			effectivePolicyReconciler.Reconcile,
		},
		Postcondition: (&reconcilers.TopologyFileReconciler{}).Reconcile, // Graphiz frees the memory that might be simutanously used by the reconcilers, so this needs to run in a precondition
	}

	return reconciler.Run
}

func controllerOptionsFor(gatewayProviders []string) []controller.ControllerOptionFunc {
	var opts []controller.ControllerOptionFunc

	// if we care about specificities of gateway controllers, then let's add gateway classes to the topology too
	if len(gatewayProviders) > 0 {
		opts = append(opts, controller.WithRunnable("gatewayclass informer", controller.Watch[*gwapiv1.GatewayClass](controller.GatewayClassesResource, metav1.NamespaceNone)))
	}

	for _, gatewayProvider := range gatewayProviders {
		switch gatewayProvider {
		case reconcilers.EnvoyGatewayProviderName:
			opts = append(opts, controller.WithRunnable("envoygateway/securitypolicy informer", controller.Watch[*egv1alpha1.SecurityPolicy](reconcilers.EnvoyGatewaySecurityPoliciesResource, metav1.NamespaceAll)))
			opts = append(opts, controller.WithObjectKinds(reconcilers.EnvoyGatewaySecurityPolicyKind))
			opts = append(opts, controller.WithObjectLinks(reconcilers.LinkGatewayToEnvoyGatewaySecurityPolicyFunc))
		case reconcilers.IstioGatewayProviderName:
			opts = append(opts, controller.WithRunnable("istio/authorizationpolicy informer", controller.Watch[*istiov1.AuthorizationPolicy](reconcilers.IstioAuthorizationPoliciesResource, metav1.NamespaceAll)))
			opts = append(opts, controller.WithObjectKinds(reconcilers.IstioAuthorizationPolicyKind))
			opts = append(opts, controller.WithObjectLinks(reconcilers.LinkGatewayToIstioAuthorizationPolicyFunc))
		}
	}

	return opts
}
