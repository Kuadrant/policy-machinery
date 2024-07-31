package reconcilers

import (
	"context"
	"log"
	"strings"

	egv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"
)

const EnvoyGatewayProviderName = "envoygateway"

var (
	EnvoyGatewaySecurityPolicyKind       = schema.GroupKind{Group: egv1alpha1.GroupName, Kind: "SecurityPolicy"}
	EnvoyGatewaySecurityPoliciesResource = egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies")
)

type EnvoyGatewayProvider struct {
	Client *dynamic.DynamicClient
}

func (p *EnvoyGatewayProvider) ReconcileSecurityPolicies(ctx context.Context, resourceEvent controller.ResourceEvent, topology *machinery.Topology) {
	authPaths := pathsFromContext(ctx, authPathsKey)
	targetables := topology.Targetables()
	gateways := targetables.Items(func(o machinery.Object) bool {
		_, ok := o.(*machinery.Gateway)
		return ok
	})
	for _, gateway := range gateways {
		paths := lo.Filter(authPaths, func(path []machinery.Targetable, _ int) bool {
			if len(path) < 4 { // should never happen
				log.Fatalf("Unexpected topology path length to build Envoy SecurityPolicy: %s\n", strings.Join(lo.Map(path, machinery.MapTargetableToURLFunc), " → "))
			}
			return path[0].GetURL() == gateway.GetURL() && lo.ContainsBy(targetables.Parents(path[0]), func(parent machinery.Targetable) bool {
				gc, ok := parent.(*machinery.GatewayClass)
				return ok && gc.Spec.ControllerName == "gateway.envoyproxy.io/gatewayclass-controller"
			})
		})
		if len(paths) > 0 {
			p.createSecurityPolicy(ctx, topology, gateway)
			continue
		}
		p.deleteSecurityPolicy(ctx, topology, gateway.GetNamespace(), gateway.GetName(), gateway)
	}
}

func (p *EnvoyGatewayProvider) DeleteSecurityPolicy(ctx context.Context, resourceEvent controller.ResourceEvent, topology *machinery.Topology) {
	gateway := resourceEvent.OldObject
	p.deleteSecurityPolicy(ctx, topology, gateway.GetNamespace(), gateway.GetName(), nil)
}

func (p *EnvoyGatewayProvider) createSecurityPolicy(ctx context.Context, topology *machinery.Topology, gateway machinery.Targetable) {
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
					BackendRef: &gwapiv1.BackendObjectReference{
						Name:      gwapiv1.ObjectName("authorino-authorino-authorization"),
						Namespace: ptr.To(gwapiv1.Namespace("kuadrant-system")),
						Port:      ptr.To(gwapiv1.PortNumber(50051)),
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
			log.Println("failed to create SecurityPolicy", err)
		}
		return
	}

	securityPolicy := obj.(*controller.Object).RuntimeObject.(*egv1alpha1.SecurityPolicy)

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
		log.Println("failed to update SecurityPolicy", err)
	}
}

func (p *EnvoyGatewayProvider) deleteSecurityPolicy(ctx context.Context, topology *machinery.Topology, namespace, name string, parent machinery.Targetable) {
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
		return
	}
	resource := p.Client.Resource(EnvoyGatewaySecurityPoliciesResource).Namespace(namespace)
	err := resource.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Println("failed to delete SecurityPolicy", err)
	}
}

func LinkGatewayToEnvoyGatewaySecurityPolicyFunc(objs controller.Store) machinery.LinkFunc {
	gatewayKind := machinery.GatewayGroupKind
	gateways := lo.FilterMap(lo.Values(objs[gatewayKind]), func(obj controller.RuntimeObject, _ int) (*gwapiv1.Gateway, bool) {
		g, ok := obj.(*gwapiv1.Gateway)
		if !ok {
			return nil, false
		}
		return g, true
	})

	return machinery.LinkFunc{
		From: gatewayKind,
		To:   EnvoyGatewaySecurityPolicyKind,
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
