package machinery

import (
	"bytes"
	"fmt"
	"strings"

	graphviz "github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Topology models a network of related targetables and respective policies attached to them.
type Topology struct {
	graph       *cgraph.Graph
	targetables map[string]Targetable
	policies    map[string]Policy
}

type FilterFunc func(Object) bool

// Targetables returns all targetable nodes of a given kind in the topology.
func (t *Topology) Targetables(filters ...FilterFunc) []Targetable {
	return lo.Filter(lo.Values(t.targetables), func(targetable Targetable, _ int) bool {
		o := targetable.(Object)
		for _, f := range filters {
			if !f(o) {
				return false
			}
		}
		return true
	})
}

// Policies returns all policies of a given kind in the topology.
func (t *Topology) Policies(filters ...FilterFunc) []Policy {
	return lo.Filter(lo.Values(t.policies), func(policy Policy, _ int) bool {
		o := policy.(Object)
		for _, f := range filters {
			if !f(o) {
				return false
			}
		}
		return true
	})
}

func (t *Topology) ToDot() string {
	gz := graphviz.New()
	var buf bytes.Buffer
	gz.Render(t.graph, "dot", &buf)
	return buf.String()
}

// NewTopology returns a network of targetable Gateway API nodes, from a list of related Gateway API resources
// and attached policies.
// The topology is represented as a directed acyclic graph (DAG) with the following structure:
//
//	GatewayClass -> Gateway -> Listener -> HTTPRoute -> HTTPRouteRule -> Backend -> BackendPort
//	                                                                  ∟> BackendPort <- Backend
func NewTopology(targetables []Targetable, policies []Policy) *Topology {
	policiesByTargetRef := make(map[string][]Policy)
	for i := range policies {
		policy := policies[i]
		for _, targetRef := range policy.GetTargetRefs() {
			if policiesByTargetRef[targetRef.GetURL()] == nil {
				policiesByTargetRef[targetRef.GetURL()] = make([]Policy, 0)
			}
			policiesByTargetRef[targetRef.GetURL()] = append(policiesByTargetRef[targetRef.GetURL()], policy)
		}
	}

	targetables = lo.Map(targetables, func(t Targetable, _ int) Targetable {
		t.SetPolicies(policiesByTargetRef[t.GetURL()])
		return t
	})

	gz := graphviz.New()
	graph, _ := gz.Graph(graphviz.StrictDirected)

	gatewayClasses := filterTargetablesByKind[GatewayClass](targetables)
	addTargetablesToGraph(graph, gatewayClasses)
	gateways := filterTargetablesByKind[Gateway](targetables)
	addTargetablesToGraph(graph, gateways)
	listeners := lo.FlatMap(gateways, listenersFromGatewayFunc)
	addTargetablesToGraph(graph, listeners)
	httpRoutes := filterTargetablesByKind[HTTPRoute](targetables)
	addTargetablesToGraph(graph, httpRoutes)
	httpRouteRules := lo.FlatMap(httpRoutes, httpRouteRulesFromHTTPRouteFunc)
	addTargetablesToGraph(graph, httpRouteRules)
	backends := filterTargetablesByKind[Backend](targetables)
	addTargetablesToGraph(graph, backends)
	backendPorts := lo.FlatMap(backends, backendPortsFromBackendFunc)
	addTargetablesToGraph(graph, backendPorts)

	// GatewayClass -> Gateway edges
	for i := range gateways {
		gateway := gateways[i]
		gatewayClass, ok := lo.Find(gatewayClasses, func(gc GatewayClass) bool {
			return gc.Name == string(gateway.Spec.GatewayClassName)
		})
		if ok {
			addEdgeToGraph(graph, "GatewayClass -> Gateway", gatewayClass, gateway)
		}
	}
	// Gateway -> Listener edges
	for i := range listeners {
		listener := listeners[i]
		addEdgeToGraph(graph, "Gateway -> Listener", listener.gateway, listener)
	}
	// Listener -> HTTPRoute edges
	for i := range httpRoutes {
		httpRoute := httpRoutes[i]
		parentListeners := lo.FlatMap(httpRoute.Spec.ParentRefs, func(parentRef gwapiv1.ParentReference, _ int) []Listener {
			if (parentRef.Group != nil && parentRef.Group != ptr.To(gwapiv1.Group(gwapiv1.GroupName))) || (parentRef.Kind != nil && parentRef.Kind != ptr.To(gwapiv1.Kind("Gateway"))) {
				return nil
			}
			gatewayNamespace := string(ptr.Deref(parentRef.Namespace, gwapiv1.Namespace(httpRoute.Namespace)))
			gateway, ok := lo.Find(gateways, func(g Gateway) bool {
				return g.Namespace == gatewayNamespace && g.Name == string(parentRef.Name)
			})
			if !ok {
				return nil
			}
			if parentRef.SectionName != nil {
				listener, ok := lo.Find(listeners, func(l Listener) bool {
					return l.gateway.GetURL() == gateway.GetURL() && l.Name == *parentRef.SectionName
				})
				if !ok {
					return nil
				}
				return []Listener{listener}
			}
			return lo.Filter(listeners, func(l Listener, _ int) bool {
				return l.gateway.GetURL() == gateway.GetURL()
			})
		})
		for _, listener := range parentListeners {
			addEdgeToGraph(graph, "Listener -> HTTPRoute", listener, httpRoute)
		}
	}
	// HTTPRoute -> HTTPRouteRule edges
	for i := range httpRouteRules {
		httpRouteRule := httpRouteRules[i]
		addEdgeToGraph(graph, "HTTPRoute -> HTTPRouteRule", httpRouteRule.httpRoute, httpRouteRule)
	}
	// Backend -> BackendPort edges
	for i := range backendPorts {
		backendPort := backendPorts[i]
		addEdgeToGraph(graph, "Backend -> BackendPort", backendPort.backend, backendPort)
	}
	// HTTPRouteRule -> (Backend|BackendPort) edges
	for i := range httpRouteRules {
		httpRouteRule := httpRouteRules[i]
		for _, backendRef := range httpRouteRule.BackendRefs {
			backendNamespace := string(ptr.Deref(backendRef.Namespace, gwapiv1.Namespace(httpRouteRule.httpRoute.Namespace)))
			backend, ok := lo.Find(backends, func(b Backend) bool {
				return b.Namespace == backendNamespace && b.Name == string(backendRef.Name)
			})
			if !ok {
				continue
			}
			if backendRef.Port != nil {
				backendPort, found := lo.Find(backendPorts, func(backendPort BackendPort) bool {
					return backendPort.backend.GetURL() == backend.GetURL() && backendPort.Port == int32(*backendRef.Port)
				})
				if found {
					addEdgeToGraph(graph, "HTTPRouteRule -> BackendPort", httpRouteRule, backendPort)
					continue
				}
			}
			addEdgeToGraph(graph, "HTTPRouteRule -> Backend", httpRouteRule, backend)
		}
	}

	addPoliciesToGraph(graph, policies)

	return &Topology{
		graph:       graph,
		targetables: lo.SliceToMap(targetables, associateURL[Targetable]),
		policies:    lo.SliceToMap(policies, associateURL[Policy]),
	}
}

func filterTargetablesByKind[T Targetable](targetables []Targetable) []T {
	return lo.FilterMap[Targetable, T](targetables, func(targetable Targetable, _ int) (T, bool) {
		t, ok := targetable.(T)
		return t, ok
	})
}

func listenersFromGatewayFunc(gateway Gateway, _ int) []Listener {
	return lo.Map(gateway.Spec.Listeners, func(listener gwapiv1.Listener, _ int) Listener {
		return Listener{
			Listener: &listener,
			gateway:  &gateway,
		}
	})
}

func httpRouteRulesFromHTTPRouteFunc(httpRoute HTTPRoute, _ int) []HTTPRouteRule {
	return lo.Map(httpRoute.Spec.Rules, func(rule gwapiv1.HTTPRouteRule, i int) HTTPRouteRule {
		return HTTPRouteRule{
			HTTPRouteRule: &rule,
			httpRoute:     &httpRoute,
			name:          gwapiv1.SectionName(fmt.Sprintf("rule-%d", i+1)),
		}
	})
}

func backendPortsFromBackendFunc(backend Backend, _ int) []BackendPort {
	return lo.Map(backend.Spec.Ports, func(port core.ServicePort, _ int) BackendPort {
		return BackendPort{
			ServicePort: &port,
			backend:     &backend,
		}
	})
}

func associateURL[T Object](obj T) (string, T) {
	return obj.GetURL(), obj
}

func addObjectsToGraph[T Object](graph *cgraph.Graph, objects []T, shape cgraph.Shape) []*cgraph.Node {
	return lo.Map(objects, func(object T, _ int) *cgraph.Node {
		name := strings.TrimPrefix(namespacedName(object.GetNamespace(), object.GetName()), "/")
		n, _ := graph.CreateNode(string(object.GetURL()))
		n.SetLabel(fmt.Sprintf("%s\\n%s", object.GroupVersionKind().Kind, name))
		n.SetShape(shape)
		return n
	})
}

func addTargetablesToGraph[T Targetable](graph *cgraph.Graph, targetables []T) {
	addObjectsToGraph(graph, targetables, cgraph.BoxShape)
}

func addPoliciesToGraph[T Policy](graph *cgraph.Graph, policies []T) {
	for i, policyNode := range addObjectsToGraph(graph, policies, cgraph.EllipseShape) {
		// Policy -> Target edges
		for _, targetRef := range policies[i].GetTargetRefs() {
			targetNode, _ := graph.Node(string(targetRef.GetURL()))
			if targetNode != nil {
				graph.CreateEdge("Policy -> Target", policyNode, targetNode)
			}
		}
	}
}

func addEdgeToGraph(graph *cgraph.Graph, name string, parent, child Targetable) {
	p, _ := graph.CreateNode(string(parent.GetURL()))
	c, _ := graph.CreateNode(string(child.GetURL()))
	if p != nil && c != nil {
		graph.CreateEdge(name, p, c)
	}
}

func objectKindFromBackendRef(backendRef gwapiv1.BackendObjectReference) schema.ObjectKind {
	return &metav1.TypeMeta{
		Kind:       string(ptr.Deref(backendRef.Kind, gwapiv1.Kind("Service"))),
		APIVersion: string(ptr.Deref(backendRef.Group, gwapiv1.Group(""))) + "/",
	}
}
