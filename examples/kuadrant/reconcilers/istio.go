package reconcilers

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

const IstioGatewayProviderName = "istio"

var (
	IstioAuthorizationPolicyKind       = schema.GroupKind{Group: istiov1.GroupName, Kind: "AuthorizationPolicy"}
	IstioAuthorizationPoliciesResource = istiov1.SchemeGroupVersion.WithResource("authorizationpolicies")
)

type IstioGatewayProvider struct {
	Client *dynamic.DynamicClient
}

func (p *IstioGatewayProvider) ReconcileAuthorizationPolicies(ctx context.Context, resourceEvent controller.ResourceEvent, topology *machinery.Topology) {
	authPaths := pathsFromContext(ctx, authPathsKey)
	targetables := topology.Targetables()
	gateways := targetables.Items(func(o machinery.Object) bool {
		_, ok := o.(*machinery.Gateway)
		return ok
	})
	for _, gateway := range gateways {
		paths := lo.Filter(authPaths, func(path []machinery.Targetable, _ int) bool {
			if len(path) < 4 { // should never happen
				log.Fatalf("Unexpected topology path length to build Istio AuthorizationPolicy: %s\n", strings.Join(lo.Map(path, machinery.MapTargetableToURLFunc), " → "))
			}
			return path[0].GetURL() == gateway.GetURL() && lo.ContainsBy(targetables.Parents(path[0]), func(parent machinery.Targetable) bool {
				gc, ok := parent.(*machinery.GatewayClass)
				return ok && gc.Spec.ControllerName == "istio.io/gateway-controller"
			})
		})
		if len(paths) > 0 {
			p.createAuthorizationPolicy(ctx, topology, gateway, paths)
			continue
		}
		p.deleteAuthorizationPolicy(ctx, topology, gateway.GetNamespace(), gateway.GetName(), gateway)
	}
}

func (p *IstioGatewayProvider) DeleteAuthorizationPolicy(ctx context.Context, resourceEvent controller.ResourceEvent, topology *machinery.Topology) {
	gateway := resourceEvent.OldObject
	p.deleteAuthorizationPolicy(ctx, topology, gateway.GetNamespace(), gateway.GetName(), nil)
}

func (p *IstioGatewayProvider) createAuthorizationPolicy(ctx context.Context, topology *machinery.Topology, gateway machinery.Targetable, paths [][]machinery.Targetable) {
	desiredAuthorizationPolicy := &istiov1.AuthorizationPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: istiov1.SchemeGroupVersion.String(),
			Kind:       IstioAuthorizationPolicyKind.Kind,
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
		listener := path[1].(*machinery.Listener)
		httpRoute := path[2].(*machinery.HTTPRoute)
		routeRule := path[3].(*machinery.HTTPRouteRule)
		hostname := ptr.Deref(listener.Hostname, gwapiv1.Hostname("*"))
		hostnames := []gwapiv1.Hostname{hostname}
		if len(httpRoute.Spec.Hostnames) > 0 {
			hostnames = lo.Filter(httpRoute.Spec.Hostnames, hostSubsetOf(hostname))
		}
		rules := istioAuthorizationPolicyRulesFromHTTPRouteRule(routeRule.HTTPRouteRule, hostnames)
		desiredAuthorizationPolicy.Spec.Rules = append(desiredAuthorizationPolicy.Spec.Rules, rules...)
	}

	resource := p.Client.Resource(IstioAuthorizationPoliciesResource).Namespace(gateway.GetNamespace())

	obj, found := lo.Find(topology.Objects().Children(gateway), func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == IstioAuthorizationPolicyKind && o.GetNamespace() == gateway.GetNamespace() && o.GetName() == gateway.GetName()
	})

	if !found {
		o, _ := controller.Destruct(desiredAuthorizationPolicy)
		_, err := resource.Create(ctx, o, metav1.CreateOptions{})
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
	_, err := resource.Update(ctx, o, metav1.UpdateOptions{})
	if err != nil {
		log.Println("failed to update AuthorizationPolicy", err)
	}
}

func (p *IstioGatewayProvider) deleteAuthorizationPolicy(ctx context.Context, topology *machinery.Topology, namespace, name string, parent machinery.Targetable) {
	var objs []machinery.Object
	if parent != nil {
		objs = topology.Objects().Children(parent)
	} else {
		objs = topology.Objects().Items()
	}
	_, found := lo.Find(objs, func(o machinery.Object) bool {
		return o.GroupVersionKind().GroupKind() == IstioAuthorizationPolicyKind && o.GetNamespace() == namespace && o.GetName() == name
	})
	if !found {
		return
	}
	resource := p.Client.Resource(IstioAuthorizationPoliciesResource).Namespace(namespace)
	err := resource.Delete(ctx, name, metav1.DeleteOptions{})
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

// TODO: move this to a shared package
func hostSubsetOf(superset gwapiv1.Hostname) func(gwapiv1.Hostname, int) bool {
	wildcarded := func(hostname gwapiv1.Hostname) bool {
		return len(hostname) > 0 && hostname[0] == '*'
	}

	return func(hostname gwapiv1.Hostname, _ int) bool {
		if wildcarded(hostname) {
			if wildcarded(superset) {
				// both hostname and superset contain wildcard
				if len(hostname) < len(superset) {
					return false
				}
				return strings.HasSuffix(string(hostname[1:]), string(superset[1:]))
			}
			// only hostname contains wildcard
			return false
		}

		if wildcarded(superset) {
			// only superset contains wildcard
			return strings.HasSuffix(string(hostname), string(superset[1:]))
		}

		// neither contains wildcard, so do normal string comparison
		return hostname == superset
	}
}

func LinkGatewayToIstioAuthorizationPolicyFunc(objs controller.Store) machinery.LinkFunc {
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
		To:   IstioAuthorizationPolicyKind,
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
