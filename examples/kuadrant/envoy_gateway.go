package main

import (
	"context"
	"log"

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

const envoyGatewayProvider = "envoygateway"

var (
	_ GatewayProvider = &EnvoyGatewayProvider{}

	envoyGatewaySecurityPolicyKind       = schema.GroupKind{Group: egv1alpha1.GroupName, Kind: "SecurityPolicy"}
	envoyGatewaySecurityPoliciesResource = egv1alpha1.SchemeBuilder.GroupVersion.WithResource("securitypolicies")
)

type EnvoyGatewayProvider struct {
	*dynamic.DynamicClient
}

func (p *EnvoyGatewayProvider) ReconcileGateway(topology *machinery.Topology, gateway machinery.Targetable, capabilities map[string][][]machinery.Targetable) {
	// check if the gateway is managed by the envoy gateway controller
	if !lo.ContainsBy(topology.Targetables().Parents(gateway), func(p machinery.Targetable) bool {
		gc, ok := p.(*machinery.GatewayClass)
		return ok && gc.Spec.ControllerName == "gateway.envoyproxy.io/gatewayclass-controller"
	}) {
		return
	}

	// reconcile envoy gateway securitypolicy resources
	if lo.ContainsBy(capabilities["auth"], func(path []machinery.Targetable) bool {
		return lo.Contains(lo.Map(path, machinery.MapTargetableToURLFunc), gateway.GetURL())
	}) {
		p.createSecurityPolicy(topology, gateway)
		return
	}
	p.deleteSecurityPolicy(topology, gateway)
}

func (p *EnvoyGatewayProvider) createSecurityPolicy(topology *machinery.Topology, gateway machinery.Targetable) {
	desiredSecurityPolicy := &egv1alpha1.SecurityPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: egv1alpha1.GroupVersion.String(),
			Kind:       envoyGatewaySecurityPolicyKind.Kind,
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

	resource := p.Resource(envoyGatewaySecurityPoliciesResource).Namespace(gateway.GetNamespace())

	obj, found := lo.Find(topology.Objects().Children(gateway), func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == envoyGatewaySecurityPolicyKind && o.GetNamespace() == gateway.GetNamespace() && o.GetName() == gateway.GetName()
	})

	if !found {
		o, _ := controller.Destruct(desiredSecurityPolicy)
		_, err := resource.Create(context.TODO(), o, metav1.CreateOptions{})
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
	_, err := resource.Update(context.TODO(), o, metav1.UpdateOptions{})
	if err != nil {
		log.Println("failed to update SecurityPolicy", err)
	}
}

func (p *EnvoyGatewayProvider) deleteSecurityPolicy(topology *machinery.Topology, gateway machinery.Targetable) {
	_, found := lo.Find(topology.Objects().Children(gateway), func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == envoyGatewaySecurityPolicyKind && o.GetNamespace() == gateway.GetNamespace() && o.GetName() == gateway.GetName()
	})

	if !found {
		return
	}

	resource := p.Resource(envoyGatewaySecurityPoliciesResource).Namespace(gateway.GetNamespace())
	err := resource.Delete(context.TODO(), gateway.GetName(), metav1.DeleteOptions{})
	if err != nil {
		log.Println("failed to delete SecurityPolicy", err)
	}
}

func linkGatewayToEnvoyGatewaySecurityPolicyFunc(objs controller.Store) machinery.LinkFunc {
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
		To:   envoyGatewaySecurityPolicyKind,
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
