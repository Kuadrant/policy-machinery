package reconcilers

import (
	"context"
	"fmt"
	"sync"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const EnvoyGatewayProviderName = "envoygateway"

var (
	EnvoyGatewaySecurityPolicyKind       = schema.GroupKind{Group: egv1alpha1.GroupName, Kind: "SecurityPolicy"}
	EnvoyGatewaySecurityPoliciesResource = egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies")
)

type EnvoyGatewayProvider struct {
	Client *dynamic.DynamicClient
}

func (p *EnvoyGatewayProvider) ReconcileSecurityPolicies(ctx context.Context, _ []controller.ResourceEvent, topology *machinery.Topology, err error, state *sync.Map) error {
	tracer := controller.TracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "envoyGatewayProvider.ReconcileSecurityPolicies")
	defer span.End()

	logger := controller.TraceLoggerFromContext(ctx).WithName("envoy gateway").WithName("securitypolicy")
	ctx = controller.LoggerIntoContext(ctx, logger)

	var authPaths [][]machinery.Targetable
	if untypedAuthPaths, ok := state.Load(authPathsKey); ok {
		authPaths = untypedAuthPaths.([][]machinery.Targetable)
	}
	targetables := topology.Targetables()
	gateways := targetables.Items(func(o machinery.Object) bool {
		_, ok := o.(*machinery.Gateway)
		return ok
	})
	for _, gateway := range gateways {
		paths := lo.Filter(authPaths, func(path []machinery.Targetable, _ int) bool {
			if len(path) != 4 { // should never happen
				err := fmt.Errorf("unexpected topology path length to build Envoy SecurityPolicy")
				span.RecordError(err)
				logger.Error(err, "invalid topology path",
					"gateway.name", gateway.GetName(),
					"gateway.namespace", gateway.GetNamespace(),
					"path", lo.Map(path, machinery.MapTargetableToLocatorFunc),
				)
				return false
			}
			return path[0].GetLocator() == gateway.GetLocator() && lo.ContainsBy(targetables.Parents(path[0]), func(parent machinery.Targetable) bool {
				gc, ok := parent.(*machinery.GatewayClass)
				return ok && gc.Spec.ControllerName == "gateway.envoyproxy.io/gatewayclass-controller"
			})
		})
		if len(paths) > 0 {
			logger.V(1).Info("reconciling security policy",
				"gateway.name", gateway.GetName(),
				"gateway.namespace", gateway.GetNamespace(),
				"paths.count", len(paths),
			)
			span.AddEvent("creating security policy", trace.WithAttributes(
				attribute.String("gateway.name", gateway.GetName()),
				attribute.String("gateway.namespace", gateway.GetNamespace()),
				attribute.Int("paths.count", len(paths))),
			)
			p.createSecurityPolicy(ctx, topology, gateway)
			continue
		}
		logger.V(1).Info("deleting security policy",
			"gateway.name", gateway.GetName(),
			"gateway.namespace", gateway.GetNamespace(),
		)
		span.AddEvent("deleting security policy", trace.WithAttributes(
			attribute.String("gateway.name", gateway.GetName()),
			attribute.String("gateway.namespace", gateway.GetNamespace())),
		)
		p.deleteSecurityPolicy(ctx, topology, gateway.GetNamespace(), gateway.GetName(), gateway)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (p *EnvoyGatewayProvider) DeleteSecurityPolicy(ctx context.Context, resourceEvents []controller.ResourceEvent, topology *machinery.Topology, err error, _ *sync.Map) error {
	tracer := controller.TracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "envoyGatewayProvider.DeleteSecurityPolicy")
	defer span.End()

	logger := controller.TraceLoggerFromContext(ctx).WithName("envoy gateway").WithName("securitypolicy")
	ctx = controller.LoggerIntoContext(ctx, logger)

	for _, resourceEvent := range resourceEvents {
		gateway := resourceEvent.OldObject
		logger.V(1).Info("processing gateway deletion event",
			"gateway.name", gateway.GetName(),
			"gateway.namespace", gateway.GetNamespace(),
		)
		span.AddEvent("deleting security policy", trace.WithAttributes(
			attribute.String("gateway.name", gateway.GetName()),
			attribute.String("gateway.namespace", gateway.GetNamespace())),
		)
		p.deleteSecurityPolicy(ctx, topology, gateway.GetNamespace(), gateway.GetName(), nil)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (p *EnvoyGatewayProvider) createSecurityPolicy(ctx context.Context, topology *machinery.Topology, gateway machinery.Targetable) {
	logger := controller.LoggerFromContext(ctx)

	desiredSecurityPolicy := &egv1alpha1.SecurityPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: egv1alpha1.GroupVersion.String(),
			Kind:       EnvoyGatewaySecurityPolicyKind.Kind,
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
					BackendCluster: egv1alpha1.BackendCluster{
						BackendRefs: []egv1alpha1.BackendRef{
							{
								BackendObjectReference: gwapiv1.BackendObjectReference{
									Name:      gwapiv1.ObjectName("authorino-authorino-authorization"),
									Namespace: ptr.To(gwapiv1.Namespace("kuadrant-system")),
									Port:      ptr.To(gwapiv1.PortNumber(50051)),
								},
							},
						},
					},
				},
			},
		},
	}

	resource := p.Client.Resource(EnvoyGatewaySecurityPoliciesResource).Namespace(gateway.GetNamespace())

	obj, found := lo.Find(topology.Objects().Children(gateway), func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == EnvoyGatewaySecurityPolicyKind && o.GetNamespace() == gateway.GetNamespace() && o.GetName() == gateway.GetName()
	})

	if !found {
		o, _ := controller.Destruct(desiredSecurityPolicy)
		_, err := resource.Create(ctx, o, metav1.CreateOptions{})
		if err != nil {
			span := trace.SpanFromContext(ctx)
			span.RecordError(err)
			logger.Error(err, "failed to create SecurityPolicy",
				"gateway.name", gateway.GetName(),
				"gateway.namespace", gateway.GetNamespace(),
			)
		} else {
			logger.Info("created SecurityPolicy",
				"gateway.name", gateway.GetName(),
				"gateway.namespace", gateway.GetNamespace(),
			)
		}
		return
	}

	securityPolicy := obj.(*controller.RuntimeObject).Object.(*egv1alpha1.SecurityPolicy)

	if securityPolicy.Spec.ExtAuth != nil &&
		securityPolicy.Spec.ExtAuth.GRPC != nil &&
		securityPolicy.Spec.ExtAuth.GRPC.BackendRef != nil &&
		securityPolicy.Spec.ExtAuth.GRPC.BackendRef.Namespace != nil &&
		*securityPolicy.Spec.ExtAuth.GRPC.BackendRef.Namespace == *desiredSecurityPolicy.Spec.ExtAuth.GRPC.BackendRef.Namespace &&
		securityPolicy.Spec.ExtAuth.GRPC.BackendRef.Name == desiredSecurityPolicy.Spec.ExtAuth.GRPC.BackendRef.Name &&
		securityPolicy.Spec.ExtAuth.GRPC.BackendRef.Port != nil &&
		*securityPolicy.Spec.ExtAuth.GRPC.BackendRef.Port == *desiredSecurityPolicy.Spec.ExtAuth.GRPC.BackendRef.Port {
		return
	}

	securityPolicy.Spec = desiredSecurityPolicy.Spec
	o, _ := controller.Destruct(securityPolicy)
	_, err := resource.Update(ctx, o, metav1.UpdateOptions{})
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err)
		logger.Error(err, "failed to update SecurityPolicy",
			"gateway.name", gateway.GetName(),
			"gateway.namespace", gateway.GetNamespace(),
		)
	} else {
		logger.Info("updated SecurityPolicy",
			"gateway.name", gateway.GetName(),
			"gateway.namespace", gateway.GetNamespace(),
		)
	}
}

func (p *EnvoyGatewayProvider) deleteSecurityPolicy(ctx context.Context, topology *machinery.Topology, namespace, name string, parent machinery.Targetable) {
	logger := controller.LoggerFromContext(ctx)

	var objs []machinery.Object
	if parent != nil {
		objs = topology.Objects().Children(parent)
	} else {
		objs = topology.Objects().Items()
	}
	_, found := lo.Find(objs, func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == EnvoyGatewaySecurityPolicyKind && o.GetNamespace() == namespace && o.GetName() == name
	})
	if !found {
		logger.V(1).Info("SecurityPolicy not found, skipping deletion",
			"name", name,
			"namespace", namespace,
		)
		return
	}
	resource := p.Client.Resource(EnvoyGatewaySecurityPoliciesResource).Namespace(namespace)
	err := resource.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err)
		logger.Error(err, "failed to delete SecurityPolicy",
			"name", name,
			"namespace", namespace,
		)
	} else {
		logger.Info("deleted SecurityPolicy",
			"name", name,
			"namespace", namespace,
		)
	}
}

func LinkGatewayToEnvoyGatewaySecurityPolicyFunc(objs controller.Store) machinery.LinkFunc {
	gateways := lo.Map(objs.FilterByGroupKind(machinery.GatewayGroupKind), controller.ObjectAs[*gwapiv1.Gateway])

	return machinery.LinkFunc{
		From: machinery.GatewayGroupKind,
		To:   EnvoyGatewaySecurityPolicyKind,
		Func: func(child machinery.Object) []machinery.Object {
			o := child.(*controller.RuntimeObject)
			sp := o.Object.(*egv1alpha1.SecurityPolicy)
			refs := sp.Spec.PolicyTargetReferences.TargetRefs
			if ref := sp.Spec.PolicyTargetReferences.TargetRef; ref != nil {
				refs = append(refs, *ref)
			}
			refs = lo.Filter(refs, func(ref gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName, _ int) bool {
				return ref.Group == gwapiv1.GroupName && ref.Kind == gwapiv1.Kind(machinery.GatewayGroupKind.Kind)
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
