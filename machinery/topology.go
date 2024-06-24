package machinery

import (
	"bytes"
	"fmt"
	"strings"

	graphviz "github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"github.com/kuadrant/kuadrant-operator/pkg/library/dag"
	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const nodeKindField dag.Field = dag.Field("kind")

type TopologyOptions struct {
	GatewayClasses []GatewayClass
	Gateways       []Gateway
	HTTPRoutes     []HTTPRoute
	Backends       []Backend
	Policies       []Policy
}

type TopologyOptionsFunc func(*TopologyOptions)

func WithGatewayClasses(gatewayClasses ...*gwapiv1.GatewayClass) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.GatewayClasses = lo.Map(gatewayClasses, func(gatewayClass *gwapiv1.GatewayClass, _ int) GatewayClass {
			return GatewayClass{GatewayClass: gatewayClass}
		})
	}
}

func WithGateways(gateways ...*gwapiv1.Gateway) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Gateways = lo.Map(gateways, func(gateway *gwapiv1.Gateway, _ int) Gateway {
			return Gateway{Gateway: gateway}
		})
	}
}

func WithHTTPRoutes(httpRoutes ...*gwapiv1.HTTPRoute) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.HTTPRoutes = lo.Map(httpRoutes, func(httpRoute *gwapiv1.HTTPRoute, _ int) HTTPRoute {
			return HTTPRoute{HTTPRoute: httpRoute}
		})
	}
}

func WithBackends(backends ...*core.Service) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Backends = lo.Map(backends, func(backend *core.Service, _ int) Backend {
			return Backend{Service: backend}
		})
	}
}

func WithPolicies(policies ...Policy) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Policies = policies
	}
}

// Topology models a network of related targetables and respective policies attached to them.
type Topology struct {
	graph *dag.DAG
	kinds map[string]struct{}
}

// Targetables returns all targetable nodes of a given kind in the topology.
func (t *Topology) Targetables(kind schema.ObjectKind) []Targetable {
	nodes := t.graph.GetNodes(nodeKindField, objectKind(kind))
	return lo.FilterMap(nodes, func(node dag.Node, _ int) (Targetable, bool) {
		targetable, ok := node.(Targetable)
		return targetable, ok
	})
}

// Policies returns all policies of a given kind in the topology.
func (t *Topology) Policies(kind schema.ObjectKind) []Policy {
	nodes := t.graph.GetNodes(nodeKindField, objectKind(kind))
	return lo.FilterMap(nodes, func(node dag.Node, _ int) (Policy, bool) {
		policy, ok := node.(Policy)
		return policy, ok
	})
}

func (t *Topology) ToDot() string {
	roots := lo.FlatMap(lo.Keys(t.kinds), func(kind string, _ int) []dag.Node {
		return lo.FilterMap(t.graph.GetNodes(nodeKindField, kind), func(node dag.Node, _ int) (dag.Node, bool) {
			return node, len(t.graph.Parents(node.ID())) == 0
		})
	})
	gz := graphviz.New()
	graph, _ := gz.Graph(graphviz.StrictDirected)
	for _, root := range roots {
		t.addNodeToGraphiz(root, graph, nil)
	}
	var buf bytes.Buffer
	gz.Render(graph, "dot", &buf)
	return buf.String()
}

func (t *Topology) addNodeToGraphiz(node dag.Node, graph *cgraph.Graph, parent *cgraph.Node) {
	n, _ := graph.CreateNode(node.ID())
	n.SetLabel(nodeIDToGraphizLabel(node.ID()))
	switch node.(type) {
	case Targetable:
		n.SetShape(cgraph.BoxShape)
	case Policy:
		n.SetShape(cgraph.EllipseShape)
	}
	if parent != nil {
		graph.CreateEdge("", parent, n)
	}
	for _, child := range t.graph.Children(node.ID()) {
		t.addNodeToGraphiz(child, graph, n)
	}
}

// NewTopology returns a network of targetable Gateway API nodes, from a list of related Gateway API resources
// and attached policies.
// The topology is represented as a directed acyclic graph (DAG) with the following structure:
//
//	GatewayClass -> Gateway -> Listener -> HTTPRoute -> HTTPRouteRule -> Backend -> BackendPort
//	                                                                  ∟> BackendPort <- Backend
func NewTopology(options ...TopologyOptionsFunc) *Topology {
	o := &TopologyOptions{}
	for _, f := range options {
		f(o)
	}

	graph := dag.NewDAG(dag.WithFieldIndexer(nodeKindField, func(node dag.Node) []dag.NodeLabel {
		t, ok := node.(schema.ObjectKind)
		if !ok {
			return nil
		}
		return []dag.NodeLabel{objectKind(t)}
	}))

	// map the policies by target reference kind > name
	policiesByTargetRef := make(map[string]map[string][]Policy)
	for i := range o.Policies {
		policy := o.Policies[i]
		for _, targetRef := range policy.GetTargetRefs() {
			targetRefKind := objectKind(targetRef)
			if policiesByTargetRef[targetRefKind] == nil {
				policiesByTargetRef[targetRefKind] = make(map[string][]Policy)
			}
			policiesByTargetRef[targetRefKind][targetRef.Name()] = append(policiesByTargetRef[targetRefKind][targetRef.Name()], policy)
		}
	}

	topology := &Topology{
		graph: graph,
		kinds: make(map[string]struct{}),
	}

	addTargetablesToTopology(topology, o.GatewayClasses, policiesByTargetRef)
	addTargetablesToTopology(topology, o.Gateways, policiesByTargetRef)
	listeners := lo.FlatMap(o.Gateways, listenersFromGatewayFunc)
	addTargetablesToTopology(topology, listeners, policiesByTargetRef)
	addTargetablesToTopology(topology, o.HTTPRoutes, policiesByTargetRef)
	httpRouteRules := lo.FlatMap(o.HTTPRoutes, httpRouteRulesFromHTTPRouteFunc)
	addTargetablesToTopology(topology, httpRouteRules, policiesByTargetRef)
	addTargetablesToTopology(topology, o.Backends, policiesByTargetRef)
	backendPorts := lo.FlatMap(o.Backends, backendPortsFromBackendFunc)
	addTargetablesToTopology(topology, backendPorts, policiesByTargetRef)

	// GatewayClass -> Gateway edges
	for i := range o.Gateways {
		gateway := o.Gateways[i]
		gatewayClass, ok := lo.Find(o.GatewayClasses, func(gc GatewayClass) bool {
			return gc.Name == string(gateway.Spec.GatewayClassName)
		})
		if ok {
			graph.AddEdge(gatewayClass.ID(), gateway.ID())
		}
	}
	// Gateway -> Listener edges
	for i := range listeners {
		listener := listeners[i]
		graph.AddEdge(listener.gateway.ID(), listener.ID())
	}
	// Listener -> HTTPRoute edges
	for i := range o.HTTPRoutes {
		httpRoute := o.HTTPRoutes[i]
		listenerIDs := lo.FlatMap(httpRoute.Spec.ParentRefs, func(parentRef gwapiv1.ParentReference, _ int) []string {
			if (parentRef.Group != nil && parentRef.Group != ptr.To(gwapiv1.Group(gwapiv1.GroupName))) || (parentRef.Kind != nil && parentRef.Kind != ptr.To(gwapiv1.Kind("Gateway"))) {
				return nil
			}
			gatewayNamespace := string(ptr.Deref(parentRef.Namespace, gwapiv1.Namespace(httpRoute.Namespace)))
			gateway, ok := lo.Find(o.Gateways, func(g Gateway) bool {
				return g.Namespace == gatewayNamespace && g.Name == string(parentRef.Name)
			})
			if !ok {
				return nil
			}
			if parentRef.SectionName != nil {
				listener, ok := lo.Find(listeners, func(l Listener) bool {
					return l.gateway.ID() == gateway.ID() && l.Name == *parentRef.SectionName
				})
				if !ok {
					return nil
				}
				return []string{listener.ID()}
			}
			return lo.Map(graph.Children(gateway.ID()), func(listener dag.Node, _ int) string { return listener.ID() })
		})
		for _, listenerID := range listenerIDs {
			graph.AddEdge(listenerID, httpRoute.ID())
		}
	}
	// HTTPRoute -> HTTPRouteRule edges
	for i := range httpRouteRules {
		httpRouteRule := httpRouteRules[i]
		graph.AddEdge(httpRouteRule.httpRoute.ID(), httpRouteRule.ID())
	}
	// Backend -> BackendPort edges
	for i := range backendPorts {
		backendPort := backendPorts[i]
		graph.AddEdge(backendPort.backend.ID(), backendPort.ID())
	}
	// HTTPRouteRule -> (Backend|BackendPort) edges
	for i := range httpRouteRules {
		httpRouteRule := httpRouteRules[i]
		for _, backendRef := range httpRouteRule.BackendRefs {
			backendKind := objectKindFromBackendRef(backendRef.BackendObjectReference)
			backendNamespace := string(ptr.Deref(backendRef.Namespace, gwapiv1.Namespace(httpRouteRule.httpRoute.Namespace)))
			backendID := nodeID(backendKind, namespacedName(backendNamespace, string(backendRef.Name)))
			if backendRef.Port != nil {
				backendPort, found := lo.Find(graph.Children(backendID), func(backendPort dag.Node) bool {
					return backendPort.(BackendPort).Port == int32(*backendRef.Port)
				})
				if found {
					graph.AddEdge(httpRouteRule.ID(), backendPort.ID())
					continue
				}
			}
			graph.AddEdge(httpRouteRule.ID(), backendID)
		}
	}

	// TODO(guicassolato): Add policies to the graph

	return topology
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
			name:          fmt.Sprintf("rule-%d", i+1),
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

func addTargetablesToTopology[T Targetable](topology *Topology, targetables []T, policiesByTargetRef map[string]map[string][]Policy) {
	for i := range targetables {
		targetable := targetables[i]
		targetableKind := objectKind(targetable)
		targetable.SetPolicies(policiesByTargetRef[targetableKind][targetable.ID()])
		topology.graph.AddNode(targetable)
		topology.kinds[targetableKind] = struct{}{}
	}
}

func objectKindFromBackendRef(backendRef gwapiv1.BackendObjectReference) schema.ObjectKind {
	return &metav1.TypeMeta{
		Kind:       string(ptr.Deref(backendRef.Kind, gwapiv1.Kind("Service"))),
		APIVersion: string(ptr.Deref(backendRef.Group, gwapiv1.Group(""))) + "/",
	}
}

func nodeIDToGraphizLabel(nodeID string) string {
	parts := strings.Split(nodeID, "#")
	kind := strings.TrimSuffix(strings.SplitAfter(parts[0], ".")[0], ".")
	name := parts[1]
	return fmt.Sprintf("%s\\n%s", kind, name)
}
