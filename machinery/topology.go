package machinery

import (
	"fmt"
	"strings"

	"github.com/emicklei/dot"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

type TopologyOptions struct {
	Targetables []Targetable
	Policies    []Policy
	Objects     []Object
	Links       []LinkFunc
}

type LinkFunc struct {
	From schema.GroupKind
	To   schema.GroupKind
	Func func(child Object) (parents []Object)
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

// WithPolicies adds policies to the options to initialize a new topology.
func WithPolicies[T Policy](policies ...T) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Policies = append(o.Policies, lo.Map(policies, func(policy T, _ int) Policy {
			return policy
		})...)
	}
}

// WithObjects adds generic objects to the options to initialize a new topology.
// Do not use this function to add targetables or policies.
// Use WithLinks to define the relationships between objects of any kind.
func WithObjects[T Object](objects ...T) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Objects = append(o.Objects, lo.Map(objects, func(object T, _ int) Object {
			return object
		})...)
	}
}

// WithLinks adds link functions to the options to initialize a new topology.
func WithLinks(links ...LinkFunc) TopologyOptionsFunc {
	return func(o *TopologyOptions) {
		o.Links = append(o.Links, links...)
	}
}

// NewTopology returns a network of targetable resources, attached policies, and other kinds of objects.
// The topology is represented as a directed acyclic graph (DAG) with the structure given by link functions.
// The links between policies to targteables are inferred from the policies' target references.
// The targetables, policies, objects and link functions are provided as options.
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
			if policiesByTargetRef[targetRef.GetLocator()] == nil {
				policiesByTargetRef[targetRef.GetLocator()] = make([]Policy, 0)
			}
			policiesByTargetRef[targetRef.GetLocator()] = append(policiesByTargetRef[targetRef.GetLocator()], policy)
		}
	}

	targetables := lo.Map(o.Targetables, func(t Targetable, _ int) Targetable {
		t.SetPolicies(policiesByTargetRef[t.GetLocator()])
		return t
	})

	graph := dot.NewGraph(dot.Directed)

	addObjectsToGraph(graph, o.Objects)
	addTargetablesToGraph(graph, targetables)

	linkables := append(o.Objects, lo.Map(targetables, AsObject[Targetable])...)
	linkables = append(linkables, lo.Map(policies, AsObject[Policy])...)

	for _, link := range o.Links {
		children := lo.Filter(linkables, func(l Object, _ int) bool {
			return l.GroupVersionKind().GroupKind() == link.To
		})
		for _, child := range children {
			for _, parent := range link.Func(child) {
				if parent != nil {
					addEdgeToGraph(graph, fmt.Sprintf("%s -> %s", link.From.Kind, link.To.Kind), parent, child)
				}
			}
		}
	}

	addPoliciesToGraph(graph, policies)

	return &Topology{
		graph:       graph,
		objects:     lo.SliceToMap(o.Objects, associateLocator[Object]),
		targetables: lo.SliceToMap(targetables, associateLocator[Targetable]),
		policies:    lo.SliceToMap(policies, associateLocator[Policy]),
	}
}

// Topology models a network of related targetables and respective policies attached to them.
type Topology struct {
	graph       *dot.Graph
	targetables map[string]Targetable
	policies    map[string]Policy
	objects     map[string]Object
}

// Targetables returns all targetable nodes in the topology.
// The list can be filtered by providing one or more filter functions.
func (t *Topology) Targetables() *collection[Targetable] {
	return &collection[Targetable]{
		topology: t,
		items:    t.targetables,
	}
}

// Policies returns all policies in the topology.
// The list can be filtered by providing one or more filter functions.
func (t *Topology) Policies() *collection[Policy] {
	return &collection[Policy]{
		topology: t,
		items:    t.policies,
	}
}

// Objects returns all non-targetable, non-policy object nodes in the topology.
// The list can be filtered by providing one or more filter functions.
func (t *Topology) Objects() *collection[Object] {
	return &collection[Object]{
		topology: t,
		items:    t.objects,
	}
}

func (t *Topology) ToDot() string {
	return t.graph.String()
}

func addObjectsToGraph[T Object](graph *dot.Graph, objects []T) []dot.Node {
	return lo.Map(objects, func(object T, _ int) dot.Node {
		name := strings.TrimPrefix(namespacedName(object.GetNamespace(), object.GetName()), string(k8stypes.Separator))
		n := graph.Node(string(object.GetLocator()))
		n.Label(fmt.Sprintf("%s\n%s", object.GroupVersionKind().Kind, name))
		n.Attr("shape", "ellipse")
		return n
	})
}

func addTargetablesToGraph[T Targetable](graph *dot.Graph, targetables []T) {
	for _, node := range addObjectsToGraph(graph, targetables) {
		node.Attrs(
			"shape", "box",
			"style", "filled",
			"fillcolor", "#e5e5e5",
		)
	}
}

func addPoliciesToGraph[T Policy](graph *dot.Graph, policies []T) {
	for i, policyNode := range addObjectsToGraph(graph, policies) {
		policyNode.Attrs(
			"shape", "note",
			"style", "dashed",
		)
		// Policy -> Target edges
		for _, targetRef := range policies[i].GetTargetRefs() {
			targetNode, found := graph.FindNodeById(string(targetRef.GetLocator()))
			if !found {
				continue
			}
			edge := graph.Edge(policyNode, targetNode)
			edge.Attr("comment", "Policy -> Target")
			edge.Dashed()
		}
	}
}

func addEdgeToGraph(graph *dot.Graph, name string, parent, child Object) {
	p, foundParent := graph.FindNodeById(string(parent.GetLocator()))
	c, foundChild := graph.FindNodeById(string(child.GetLocator()))
	if foundParent && foundChild {
		edge := graph.Edge(p, c)
		edge.Attr("comment", name)
	}
}

func associateLocator[T Object](obj T) (string, T) {
	return obj.GetLocator(), obj
}

type collection[T Object] struct {
	topology *Topology
	items    map[string]T
}

type FilterFunc func(Object) bool

// Targetables returns all targetable nodes in the collection.
// The list can be filtered by providing one or more filter functions.
func (c *collection[T]) Targetables() *collection[Targetable] {
	return &collection[Targetable]{
		topology: c.topology,
		items:    c.topology.targetables,
	}
}

// Policies returns all policies in the collection.
// The list can be filtered by providing one or more filter functions.
func (c *collection[T]) Policies() *collection[Policy] {
	return &collection[Policy]{
		topology: c.topology,
		items:    c.topology.policies,
	}
}

// Objects returns all non-targetable, non-policy object nodes in the collection.
// The list can be filtered by providing one or more filter functions.
func (c *collection[T]) Objects() *collection[Object] {
	return &collection[Object]{
		topology: c.topology,
		items:    c.topology.objects,
	}
}

// List returns all items nodes in the collection.
// The list can be filtered by providing one or more filter functions.
func (c *collection[T]) Items(filters ...FilterFunc) []T {
	return lo.Filter(lo.Values(c.items), func(item T, _ int) bool {
		for _, f := range filters {
			if !f(item) {
				return false
			}
		}
		return true
	})
}

// Roots returns all items that have no parents in the collection.
func (c *collection[T]) Roots() []T {
	return lo.Filter(lo.Values(c.items), func(item T, _ int) bool {
		return len(c.Parents(item)) == 0
	})
}

// Parents returns all parents of a given item in the collection.
func (c *collection[T]) Parents(item Object) []T {
	var parents []T
	for from, edges := range c.topology.graph.EdgesMap() {
		if !lo.ContainsBy(edges, func(edge dot.Edge) bool {
			return edge.To().ID() == item.GetLocator()
		}) {
			continue
		}
		parent, found := c.items[from]
		if !found {
			continue
		}
		parents = append(parents, parent)
	}
	return parents
}

// Children returns all children of a given item in the collection.
func (c *collection[T]) Children(item Object) []T {
	return lo.FilterMap(c.topology.graph.EdgesMap()[item.GetLocator()], func(edge dot.Edge, _ int) (T, bool) {
		child, found := c.items[edge.To().ID()]
		return child, found
	})
}

// Paths returns all paths from a source item to a destination item in the collection.
// The order of the elements in the inner slices represents a path from the source to the destination.
func (c *collection[T]) Paths(from, to Object) [][]T {
	if from == nil || to == nil {
		return nil
	}
	var paths [][]T
	var path []T
	visited := make(map[string]bool)
	c.dfs(from, to, path, &paths, visited)
	return paths
}

// dfs performs a depth-first search to find all paths from a source item to a destination item in the collection.
func (c *collection[T]) dfs(current, to Object, path []T, paths *[][]T, visited map[string]bool) {
	currentLocator := current.GetLocator()
	if visited[currentLocator] {
		return
	}
	path = append(path, c.items[currentLocator])
	visited[currentLocator] = true
	if currentLocator == to.GetLocator() {
		pathCopy := make([]T, len(path))
		copy(pathCopy, path)
		*paths = append(*paths, pathCopy)
	} else {
		for _, child := range c.Children(current) {
			c.dfs(child, to, path, paths, visited)
		}
	}
	path = path[:len(path)-1]
	visited[currentLocator] = false
}
