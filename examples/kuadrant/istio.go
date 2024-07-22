package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/samber/lo"
	istioapiv1 "istio.io/api/security/v1"
	istiov1beta1 "istio.io/api/type/v1beta1"
	istiov1 "istio.io/client-go/pkg/apis/security/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/controller"
	"github.com/kuadrant/policy-machinery/machinery"
)

const istioGatewayProvider = "istio"

var (
	_ GatewayProvider = &IstioGatewayProvider{}

	istioAuthorizationPolicyKind       = schema.GroupKind{Group: istiov1.GroupName, Kind: "AuthorizationPolicy"}
	istioAuthorizationPoliciesResource = istiov1.SchemeGroupVersion.WithResource("authorizationpolicies")
)

type IstioGatewayProvider struct {
	*dynamic.DynamicClient
}

func (p *IstioGatewayProvider) ReconcileGateway(topology *machinery.Topology, gateway machinery.Targetable, capabilities map[string][][]machinery.Targetable) {
	// check if the gateway is managed by the istio gateway controller
	if !lo.ContainsBy(topology.Targetables().Parents(gateway), func(p machinery.Targetable) bool {
		gc, ok := p.(*machinery.GatewayClass)
		return ok && gc.Spec.ControllerName == "istio.io/gateway-controller"
	}) {
		return
	}

	// reconcile istio authorizationpolicy resources
	paths := lo.Filter(capabilities["auth"], func(path []machinery.Targetable, _ int) bool {
		return lo.Contains(lo.Map(path, machinery.MapTargetableToURLFunc), gateway.GetURL())
	})
	if len(paths) > 0 {
		p.createAuthorizationPolicy(topology, gateway, paths)
		return
	}
	p.deleteAuthorizationPolicy(topology, gateway)
}

func (p *IstioGatewayProvider) createAuthorizationPolicy(topology *machinery.Topology, gateway machinery.Targetable, paths [][]machinery.Targetable) {
	desiredAuthorizationPolicy := &istiov1.AuthorizationPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: istiov1.SchemeGroupVersion.String(),
			Kind:       istioAuthorizationPolicyKind.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.GetName(),
			Namespace: gateway.GetNamespace(),
		},
		Spec: istioapiv1.AuthorizationPolicy{
			TargetRef: &istiov1beta1.PolicyTargetReference{
				Group: gwapiv1alpha2.GroupName,
				Kind:  "Gateway",
				Name:  gateway.GetName(),
			},
			Action: istioapiv1.AuthorizationPolicy_CUSTOM,
			ActionDetail: &istioapiv1.AuthorizationPolicy_Provider{
				Provider: &istioapiv1.AuthorizationPolicy_ExtensionProvider{
					Name: "kuadrant-external-authorization",
				},
			},
		},
	}

	for _, path := range paths {
		if len(path) < 4 {
			log.Printf("Unexpected topology path length to build Istio AuthorizationPolicy: %s\n", strings.Join(lo.Map(path, machinery.MapTargetableToURLFunc), " → "))
			continue
		}
		listener := path[1].(*machinery.Listener)
		routeRule := path[3].(*machinery.HTTPRouteRule)
		hostname := ptr.Deref(listener.Hostname, gwapiv1.Hostname("*"))
		rules := istioAuthorizationPolicyRulesFromHTTPRouteRule(routeRule.HTTPRouteRule, []gwapiv1.Hostname{hostname})
		desiredAuthorizationPolicy.Spec.Rules = append(desiredAuthorizationPolicy.Spec.Rules, rules...)
	}

	resource := p.Resource(istioAuthorizationPoliciesResource).Namespace(gateway.GetNamespace())

	obj, found := lo.Find(topology.Objects().Children(gateway), func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == istioAuthorizationPolicyKind && o.GetNamespace() == gateway.GetNamespace() && o.GetName() == gateway.GetName()
	})

	if !found {
		o, _ := controller.Destruct(desiredAuthorizationPolicy)
		_, err := resource.Create(context.TODO(), o, metav1.CreateOptions{})
		if err != nil {
			log.Println("failed to create AuthorizationPolicy", err)
		}
		return
	}

	authorizationPolicy := obj.(*controller.Object).RuntimeObject.(*istiov1.AuthorizationPolicy)

	if authorizationPolicy.Spec.Action == desiredAuthorizationPolicy.Spec.Action &&
		authorizationPolicy.Spec.GetProvider() != nil &&
		authorizationPolicy.Spec.GetProvider().Name == desiredAuthorizationPolicy.Spec.GetProvider().Name &&
		len(authorizationPolicy.Spec.Rules) == len(desiredAuthorizationPolicy.Spec.Rules) &&
		lo.Every(authorizationPolicy.Spec.Rules, desiredAuthorizationPolicy.Spec.Rules) {
		return
	}

	authorizationPolicy.Spec.Action = desiredAuthorizationPolicy.Spec.Action
	authorizationPolicy.Spec.ActionDetail = desiredAuthorizationPolicy.Spec.ActionDetail
	authorizationPolicy.Spec.Rules = desiredAuthorizationPolicy.Spec.Rules
	o, _ := controller.Destruct(authorizationPolicy)
	_, err := resource.Update(context.TODO(), o, metav1.UpdateOptions{})
	if err != nil {
		log.Println("failed to update AuthorizationPolicy", err)
	}
}

func (p *IstioGatewayProvider) deleteAuthorizationPolicy(topology *machinery.Topology, gateway machinery.Targetable) {
	_, found := lo.Find(topology.Objects().Children(gateway), func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == istioAuthorizationPolicyKind && o.GetNamespace() == gateway.GetNamespace() && o.GetName() == gateway.GetName()
	})

	if !found {
		return
	}

	resource := p.Resource(istioAuthorizationPoliciesResource).Namespace(gateway.GetNamespace())
	err := resource.Delete(context.TODO(), gateway.GetName(), metav1.DeleteOptions{})
	if err != nil {
		log.Println("failed to delete AuthorizationPolicy", err)
	}
}

func istioAuthorizationPolicyRulesFromHTTPRouteRule(rule *gwapiv1.HTTPRouteRule, hostnames []gwapiv1.Hostname) (istioRules []*istioapiv1.Rule) {
	hosts := []string{}
	for _, hostname := range hostnames {
		if hostname == "*" {
			continue
		}
		hosts = append(hosts, string(hostname))
	}

	// no http route matches → we only need one simple istio rule or even no rule at all
	if len(rule.Matches) == 0 {
		if len(hosts) == 0 {
			return
		}
		istioRule := &istioapiv1.Rule{
			To: []*istioapiv1.Rule_To{
				{
					Operation: &istioapiv1.Operation{
						Hosts: hosts,
					},
				},
			},
		}
		istioRules = append(istioRules, istioRule)
		return
	}

	// http route matches and possibly hostnames → we need one istio rule per http route match
	for _, match := range rule.Matches {
		istioRule := &istioapiv1.Rule{}

		var operation *istioapiv1.Operation
		method := match.Method
		path := match.Path

		if len(hosts) > 0 || method != nil || path != nil {
			operation = &istioapiv1.Operation{}
		}

		// hosts
		if len(hosts) > 0 {
			operation.Hosts = hosts
		}

		// method
		if method != nil {
			operation.Methods = []string{string(*method)}
		}

		// path
		if path != nil {
			operator := "*" // gateway api defaults to PathMatchPathPrefix
			skip := false
			if path.Type != nil {
				switch *path.Type {
				case gwapiv1.PathMatchExact:
					operator = ""
				case gwapiv1.PathMatchRegularExpression:
					// ignore this rule as it is not supported by Istio - Authorino will check it anyway
					skip = true
				}
			}
			if !skip {
				value := "/"
				if path.Value != nil {
					value = *path.Value
				}
				operation.Paths = []string{fmt.Sprintf("%s%s", value, operator)}
			}
		}

		if operation != nil {
			istioRule.To = []*istioapiv1.Rule_To{
				{Operation: operation},
			}
		}

		// headers
		if len(match.Headers) > 0 {
			istioRule.When = []*istioapiv1.Condition{}

			for idx := range match.Headers {
				header := match.Headers[idx]
				if header.Type != nil && *header.Type == gwapiv1.HeaderMatchRegularExpression {
					// skip this rule as it is not supported by Istio - Authorino will check it anyway
					continue
				}
				headerCondition := &istioapiv1.Condition{
					Key:    fmt.Sprintf("request.headers[%s]", header.Name),
					Values: []string{header.Value},
				}
				istioRule.When = append(istioRule.When, headerCondition)
			}
		}

		// query params: istio does not support query params in authorization policies, so we build them in the authconfig instead

		istioRules = append(istioRules, istioRule)
	}
	return
}

func linkGatewayToIstioAuthorizationPolicyFunc(objs controller.Store) machinery.LinkFunc {
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
		To:   istioAuthorizationPolicyKind,
		Func: func(child machinery.Object) []machinery.Object {
			o := child.(*controller.Object)
			ap := o.RuntimeObject.(*istiov1.AuthorizationPolicy)
			refs := ap.Spec.TargetRefs
			if ref := ap.Spec.TargetRef; ref != nil {
				refs = append(refs, ref)
			}
			refs = lo.Filter(refs, func(ref *istiov1beta1.PolicyTargetReference, _ int) bool {
				return ref.Group == gwapiv1.GroupName && ref.Kind == gatewayKind.Kind
			})
			if len(refs) == 0 {
				return nil
			}
			gateway, ok := lo.Find(gateways, func(g *gwapiv1.Gateway) bool {
				if g.GetNamespace() != ap.GetNamespace() {
					return false
				}
				return lo.ContainsBy(refs, func(ref *istiov1beta1.PolicyTargetReference) bool {
					return ref.Name == g.GetName()
				})
			})
			if ok {
				return []machinery.Object{&machinery.Gateway{Gateway: gateway}}
			}
			return nil
		},
	}
}
