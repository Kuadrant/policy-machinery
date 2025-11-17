package main

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	istiov1 "istio.io/client-go/pkg/apis/security/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	ctrlruntimepredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	ctrlruntime "sigs.k8s.io/controller-runtime"
	ctrlruntimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlruntimewebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"

	kuadrantv1 "github.com/kuadrant/policy-machinery/examples/kuadrant/apis/v1"
	"github.com/kuadrant/policy-machinery/examples/kuadrant/reconcilers"
)

const (
	// reconciliation modes
	defaultReconciliationMode = stateReconciliationMode
	deltaReconciliationMode   = "delta"
	stateReconciliationMode   = "state"
)

var (
	scheme = runtime.NewScheme()

	supportedReconciliationModes = []string{stateReconciliationMode, deltaReconciliationMode}
	reconciliationMode           = defaultReconciliationMode

	supportedGatewayProviders = []string{reconcilers.EnvoyGatewayProviderName, reconcilers.IstioGatewayProviderName}
	gatewayProviders          []string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kuadrantv1.AddToScheme(scheme))
	utilruntime.Must(gwapiv1.AddToScheme(scheme))
	utilruntime.Must(egv1alpha1.AddToScheme(scheme))
	utilruntime.Must(istiov1.AddToScheme(scheme))
}

func main() {
	// parse command-line flags
	for i := range os.Args {
		switch os.Args[i] {
		case "--reconciliation-mode":
			{
				defer func() {
					if recover() != nil {
						log.Fatalf("Invalid reconciliation mode. Supported (one of): %s\n", strings.Join(lo.Map(supportedReconciliationModes, func(mode string, _ int) string {
							if mode == defaultReconciliationMode {
								return mode + " (default)"
							}
							return mode
						}), ", "))
					}
				}()
				mode := os.Args[i+1]
				if !lo.Contains(supportedReconciliationModes, mode) {
					panic("")
				}
				reconciliationMode = mode
			}
		case "--gateway-providers":
			{
				defer func() {
					if recover() != nil {
						log.Fatalf("Invalid gateway provider. Supported: %s\n", strings.Join(supportedGatewayProviders, ", "))
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

	// create logger
	logger := controller.CreateAndSetLogger()

	// load kubeconfig
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	config, err := kubeconfig.ClientConfig()
	if err != nil {
		logger.Error(err, "error loading kubeconfig")
		os.Exit(1)
	}

	// create the client
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Error(err, "error creating client")
		os.Exit(1)
	}

	// base controller options
	controllerOpts := []controller.ControllerOption{
		controller.WithLogger(logger),
		controller.WithClient(client),
		controller.WithRunnable("gateway watcher", buildWatcher(
			&gwapiv1.Gateway{},
			controller.GatewaysResource,
			metav1.NamespaceAll,
			// Example of using custom transformer function
			controller.WithTransformerFunc[*gwapiv1.Gateway](controller.TransformFunc[*gwapiv1.Gateway](func(unstructuredObj *unstructured.Unstructured) {
				unstructuredObj.SetManagedFields(nil)
			})),
			controller.WithPredicates(&ctrlruntimepredicate.TypedGenerationChangedPredicate[*gwapiv1.Gateway]{})),
		),
		controller.WithRunnable("httproute watcher", buildWatcher(
			&gwapiv1.HTTPRoute{},
			controller.HTTPRoutesResource,
			metav1.NamespaceAll,
			controller.WithPredicates(&ctrlruntimepredicate.TypedGenerationChangedPredicate[*gwapiv1.HTTPRoute]{})),
		),
		controller.WithRunnable("dnspolicy watcher", buildWatcher(
			&kuadrantv1.DNSPolicy{},
			kuadrantv1.DNSPoliciesResource,
			metav1.NamespaceAll,
			controller.WithPredicates(&ctrlruntimepredicate.TypedGenerationChangedPredicate[*kuadrantv1.DNSPolicy]{})),
		),
		controller.WithRunnable("tlspolicy watcher", buildWatcher(
			&kuadrantv1.TLSPolicy{},
			kuadrantv1.TLSPoliciesResource,
			metav1.NamespaceAll,
			controller.WithPredicates(&ctrlruntimepredicate.TypedGenerationChangedPredicate[*kuadrantv1.TLSPolicy]{})),
		),
		controller.WithRunnable("authpolicy watcher", buildWatcher(
			&kuadrantv1.AuthPolicy{},
			kuadrantv1.AuthPoliciesResource,
			metav1.NamespaceAll,
			controller.WithPredicates(&ctrlruntimepredicate.TypedGenerationChangedPredicate[*kuadrantv1.AuthPolicy]{})),
		),
		controller.WithRunnable("ratelimitpolicy watcher", buildWatcher(
			&kuadrantv1.RateLimitPolicy{},
			kuadrantv1.RateLimitPoliciesResource,
			metav1.NamespaceAll,
			controller.WithPredicates(&ctrlruntimepredicate.TypedGenerationChangedPredicate[*kuadrantv1.RateLimitPolicy]{}))),
		controller.WithPolicyKinds(
			kuadrantv1.DNSPolicyGroupKind,
			kuadrantv1.TLSPolicyGroupKind,
			kuadrantv1.AuthPolicyGroupKind,
			kuadrantv1.RateLimitPolicyGroupKind,
		),
		controller.WithReconcile(buildReconciler(gatewayProviders, client)),
	}

	// create tracer (optional, based on ENABLE_TRACING env var)
	// This demonstrates how to use OpenTelemetry tracing with policy-machinery.
	if os.Getenv("ENABLE_TRACING") == "true" {
		// Configure OTLP HTTP exporter options
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint("localhost:4318"),
			otlptracehttp.WithInsecure(),
		}

		// Create OTLP HTTP exporter for traces
		exporter, err := otlptracehttp.New(context.Background(), opts...)
		if err != nil {
			logger.Error(err, "error creating trace exporter")
			os.Exit(1)
		}
		tp := trace.NewTracerProvider(
			trace.WithBatcher(exporter),
			trace.WithResource(resource.NewSchemaless(
				semconv.ServiceName("kuadrant-example"),
				semconv.ServiceVersion("dev")),
			),
		)
		otel.SetTracerProvider(tp)
		tracer := tp.Tracer("kuadrant-policy-machinery-example")
		logger.Info("tracing enabled - spans will be exported to localhost:4318")
		controllerOpts = append(controllerOpts, controller.WithTracer(tracer))
	}

	// gateway provider specific controller options
	controllerOpts = append(controllerOpts, controllerOptionsFor(gatewayProviders)...)

	// managed controller
	if reconciliationMode == stateReconciliationMode {
		manager, err := ctrlruntime.NewManager(config, ctrlruntime.Options{
			Logger:                 logger,
			Scheme:                 scheme,
			Metrics:                ctrlruntimemetrics.Options{BindAddress: ":8080"},
			WebhookServer:          ctrlruntimewebhook.NewServer(ctrlruntimewebhook.Options{Port: 9443}),
			HealthProbeBindAddress: ":8081",
			LeaderElection:         false,
			LeaderElectionID:       "ad5be859.kuadrant.io",
		})
		if err != nil {
			logger.Error(err, "Error creating manager")
			os.Exit(1)
		}
		controllerOpts = append(controllerOpts, controller.ManagedBy(manager))
	}

	// start the controller
	if err := controller.NewController(controllerOpts...).Start(ctrlruntime.SetupSignalHandler()); err != nil {
		logger.Error(err, "error starting controller")
		os.Exit(1)
	}
}

func buildWatcher[T controller.Object](obj T, resource schema.GroupVersionResource, namespace string, options ...controller.RunnableBuilderOption[T]) controller.RunnableBuilder {
	switch reconciliationMode {
	case deltaReconciliationMode:
		options = append(options, controller.Builder(controller.IncrementalInformer[T]))
	}
	return controller.Watch(obj, resource, namespace, options...)
}

func controllerOptionsFor(gatewayProviders []string) []controller.ControllerOption {
	var opts []controller.ControllerOption

	// if we care about specificities of gateway controllers, then let's add gateway classes to the topology too
	if len(gatewayProviders) > 0 {
		opts = append(opts, controller.WithRunnable("gatewayclass watcher", buildWatcher(&gwapiv1.GatewayClass{}, controller.GatewayClassesResource, metav1.NamespaceNone)))
	}

	for _, gatewayProvider := range gatewayProviders {
		switch gatewayProvider {
		case reconcilers.EnvoyGatewayProviderName:
			opts = append(opts, controller.WithRunnable("envoygateway/securitypolicy watcher", buildWatcher(
				&egv1alpha1.SecurityPolicy{},
				reconcilers.EnvoyGatewaySecurityPoliciesResource,
				metav1.NamespaceAll,
				controller.WithPredicates(&ctrlruntimepredicate.TypedGenerationChangedPredicate[*egv1alpha1.SecurityPolicy]{}))),
			)
			opts = append(opts, controller.WithObjectKinds(reconcilers.EnvoyGatewaySecurityPolicyKind))
			opts = append(opts, controller.WithObjectLinks(reconcilers.LinkGatewayToEnvoyGatewaySecurityPolicyFunc))
		case reconcilers.IstioGatewayProviderName:
			opts = append(opts, controller.WithRunnable("istio/authorizationpolicy watcher", buildWatcher(
				&istiov1.AuthorizationPolicy{},
				reconcilers.IstioAuthorizationPoliciesResource,
				metav1.NamespaceAll,
				controller.WithPredicates(&ctrlruntimepredicate.TypedGenerationChangedPredicate[*istiov1.AuthorizationPolicy]{}))),
			)
			opts = append(opts, controller.WithObjectKinds(reconcilers.IstioAuthorizationPolicyKind))
			opts = append(opts, controller.WithObjectLinks(reconcilers.LinkGatewayToIstioAuthorizationPolicyFunc))
		}
	}

	return opts
}

// buildReconciler builds a reconciler that executes the following workflow:
//  1. log event
//  2. save topology to file
//  2. effective policies
//  3. (gateway deleted) delete SecurityPolicy / (other events) reconcile SecurityPolicies
//  3. (gateway deleted) delete AuthorizationPolicy / (other events) reconcile AuthorizationPolicies
func buildReconciler(gatewayProviders []string, client *dynamic.DynamicClient) controller.ReconcileFunc {
	effectivePolicyReconciler := &controller.Workflow{
		Precondition: reconcilers.ReconcileEffectivePolicies,
	}

	commonAuthPolicyResourceEventMatchers := []controller.ResourceEventMatcher{
		{Kind: ptr.To(machinery.GatewayClassGroupKind)},
		{Kind: ptr.To(machinery.GatewayGroupKind), EventType: ptr.To(controller.CreateEvent)},
		{Kind: ptr.To(machinery.GatewayGroupKind), EventType: ptr.To(controller.UpdateEvent)},
		{Kind: ptr.To(machinery.HTTPRouteGroupKind)},
		{Kind: ptr.To(kuadrantv1.AuthPolicyGroupKind)},
	}

	for _, gatewayProvider := range gatewayProviders {
		switch gatewayProvider {
		case reconcilers.EnvoyGatewayProviderName:
			envoyGatewayProvider := &reconcilers.EnvoyGatewayProvider{Client: client}
			effectivePolicyReconciler.Tasks = append(effectivePolicyReconciler.Tasks, (&controller.Subscription{
				ReconcileFunc: envoyGatewayProvider.ReconcileSecurityPolicies,
				Events:        append(commonAuthPolicyResourceEventMatchers, controller.ResourceEventMatcher{Kind: ptr.To(reconcilers.EnvoyGatewaySecurityPolicyKind)}),
			}).Reconcile)
			effectivePolicyReconciler.Tasks = append(effectivePolicyReconciler.Tasks, (&controller.Subscription{
				ReconcileFunc: envoyGatewayProvider.DeleteSecurityPolicy,
				Events: []controller.ResourceEventMatcher{
					{Kind: ptr.To(machinery.GatewayGroupKind), EventType: ptr.To(controller.DeleteEvent)},
				},
			}).Reconcile)
		case reconcilers.IstioGatewayProviderName:
			istioGatewayProvider := &reconcilers.IstioGatewayProvider{Client: client}
			effectivePolicyReconciler.Tasks = append(effectivePolicyReconciler.Tasks, (&controller.Subscription{
				ReconcileFunc: istioGatewayProvider.ReconcileAuthorizationPolicies,
				Events:        append(commonAuthPolicyResourceEventMatchers, controller.ResourceEventMatcher{Kind: ptr.To(reconcilers.IstioAuthorizationPolicyKind)}),
			}).Reconcile)
			effectivePolicyReconciler.Tasks = append(effectivePolicyReconciler.Tasks, (&controller.Subscription{
				ReconcileFunc: istioGatewayProvider.DeleteAuthorizationPolicy,
				Events: []controller.ResourceEventMatcher{
					{Kind: ptr.To(machinery.GatewayGroupKind), EventType: ptr.To(controller.DeleteEvent)},
				},
			}).Reconcile)
		}
	}

	reconciler := &controller.Workflow{
		Precondition: func(ctx context.Context, resourceEvents []controller.ResourceEvent, topology *machinery.Topology, err error, _ *sync.Map) error {
			logger := controller.LoggerFromContext(ctx).WithName("event logger")
			for _, event := range resourceEvents {
				// log the event
				obj := event.OldObject
				if obj == nil {
					obj = event.NewObject
				}
				values := []any{
					"type", event.EventType.String(),
					"kind", obj.GetObjectKind().GroupVersionKind().Kind,
					"namespace", obj.GetNamespace(),
					"name", obj.GetName(),
				}
				if event.EventType == controller.UpdateEvent && logger.V(1).Enabled() {
					values = append(values, "diff", cmp.Diff(event.OldObject, event.NewObject))
				}
				logger.Info("new event", values...)
			}
			return nil
		},
		Tasks: []controller.ReconcileFunc{
			// Example: Wrap individual reconcilers with tracing
			controller.TraceReconcileFunc("topology-file-writer", (&reconcilers.TopologyFileReconciler{}).Reconcile),
			controller.TraceReconcileFunc("effective-policies", effectivePolicyReconciler.Run),
		},
	}

	return reconciler.Run
}
