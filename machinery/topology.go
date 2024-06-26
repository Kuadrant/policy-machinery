package machinery

import (
	"bytes"
	"fmt"
	"strings"

	graphviz "github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

type TopologyOptions struct {
	Targetables []Targetable
	Links       []LinkFunc
	Policies    []Policy
}

type LinkFunc struct {
	From schema.GroupKind
	To   schema.GroupKind
	Func func(child Targetable) (parents []Targetable)
}

type TopologyOptionsFunc func(*TopologyOptions)

// WithTargetables adds targetables to the options to initialize a new topology.
func WithTargetables[T Targetable](targetables ...T) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Targetables = append(o.Targetables, lo.Map(targetables, func(targetable T, _ int) Targetable {
			return targetable
		})...)
	}
}

// WithLinks adds link functions to the options to initialize a new topology.
func WithLinks(links ...LinkFunc) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Links = append(o.Links, links...)
	}
}

// WithPolicies adds policies to the options to initialize a new topology.
func WithPolicies(policies ...Policy) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Policies = append(o.Policies, policies...)
	}
}

// Topology models a network of related targetables and respective policies attached to them.
type Topology struct {
	graph       *cgraph.Graph
	targetables map[string]Targetable
	policies    map[string]Policy
}

type FilterFunc func(Object) bool

// Roots returns all targetables that have no parents in the topology.
func (t *Topology) Roots() []Targetable {
	return lo.Filter(lo.Values(t.targetables), func(targetable Targetable, _ int) bool {
		return len(t.Parents(targetable)) == 0
	})
}

// Parents returns all parents of a given targetable in the topology.
func (t *Topology) Parents(targetable Targetable) []Targetable {
	var parents []Targetable
	n, err := t.graph.Node(string(targetable.GetURL()))
	if err != nil {
		return nil
	}
	edge := t.graph.FirstIn(n)
	for {
		if edge == nil {
			break
		}
		_, ok := t.targetables[edge.Node().Name()]
		if ok {
			parents = append(parents, t.targetables[edge.Node().Name()])
		}
		edge = t.graph.NextIn(edge)
	}
	return parents
}

// Children returns all children of a given targetable in the topology.
func (t *Topology) Children(targetable Targetable) []Targetable {
	var children []Targetable
	n, err := t.graph.Node(string(targetable.GetURL()))
	if err != nil {
		return nil
	}
	edge := t.graph.FirstOut(n)
	for {
		if edge == nil {
			break
		}
		_, ok := t.targetables[edge.Node().Name()]
		if ok {
			children = append(children, t.targetables[edge.Node().Name()])
		}
		edge = t.graph.NextOut(edge)
	}
	return children
}

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

func (t *Topology) ToDot() *bytes.Buffer {
	gz := graphviz.New()
	var buf bytes.Buffer
	gz.Render(t.graph, "dot", &buf)
	return &buf
}

// NewTopology returns a network of targetable resources and attached policies.
// The topology is represented as a directed acyclic graph (DAG) with the structure given by link functions.
// The targetables, policies and link functions are provided as options.
func NewTopology(options ...TopologyOptionsFunc) *Topology {
	o := &TopologyOptions{}
	for _, f := range options {
		f(o)
	}

	policies := o.Policies
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

	targetables := lo.Map(o.Targetables, func(t Targetable, _ int) Targetable {
		t.SetPolicies(policiesByTargetRef[t.GetURL()])
		return t
	})

	gz := graphviz.New()
	graph, _ := gz.Graph(graphviz.StrictDirected)

	addTargetablesToGraph(graph, targetables)

	for _, link := range o.Links {
		children := lo.Filter(targetables, func(t Targetable, _ int) bool {
			return t.GroupVersionKind().GroupKind() == link.To
		})
		for _, child := range children {
			for _, parent := range link.Func(child) {
				if parent != nil {
					addTargetablesEdgeToGraph(graph, fmt.Sprintf("%s -> %s", link.From.Kind, link.To.Kind), parent, child)
				}
			}
		}
	}

	addPoliciesToGraph(graph, policies)

	return &Topology{
		graph:       graph,
		targetables: lo.SliceToMap(targetables, associateURL[Targetable]),
		policies:    lo.SliceToMap(policies, associateURL[Policy]),
	}
}

func associateURL[T Object](obj T) (string, T) {
	return obj.GetURL(), obj
}

func addObjectsToGraph[T Object](graph *cgraph.Graph, objects []T, shape cgraph.Shape) []*cgraph.Node {
	return lo.Map(objects, func(object T, _ int) *cgraph.Node {
		name := strings.TrimPrefix(namespacedName(object.GetNamespace(), object.GetName()), string(k8stypes.Separator))
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
				edge, _ := graph.CreateEdge("Policy -> Target", policyNode, targetNode)
				edge.SetStyle(cgraph.DashedEdgeStyle)
			}
		}
	}
}

func addTargetablesEdgeToGraph(graph *cgraph.Graph, name string, parent, child Targetable) {
	p, _ := graph.Node(string(parent.GetURL()))
	c, _ := graph.Node(string(child.GetURL()))
	if p != nil && c != nil {
		graph.CreateEdge(name, p, c)
	}
}
