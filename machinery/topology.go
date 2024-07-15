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
		objects:     lo.SliceToMap(o.Objects, associateURL[Object]),
		targetables: lo.SliceToMap(targetables, associateURL[Targetable]),
		policies:    lo.SliceToMap(policies, associateURL[Policy]),
	}
}

// Topology models a network of related targetables and respective policies attached to them.
type Topology struct {
	graph       *cgraph.Graph
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

func (t *Topology) ToDot() *bytes.Buffer {
	gz := graphviz.New()
	var buf bytes.Buffer
	gz.Render(t.graph, "dot", &buf)
	return &buf
}

func addObjectsToGraph[T Object](graph *cgraph.Graph, objects []T) []*cgraph.Node {
	return lo.Map(objects, func(object T, _ int) *cgraph.Node {
		name := strings.TrimPrefix(namespacedName(object.GetNamespace(), object.GetName()), string(k8stypes.Separator))
		n, _ := graph.CreateNode(string(object.GetURL()))
		n.SetLabel(fmt.Sprintf("%s\\n%s", object.GroupVersionKind().Kind, name))
		n.SetShape(cgraph.EllipseShape)
		return n
	})
}

func addTargetablesToGraph[T Targetable](graph *cgraph.Graph, targetables []T) {
	for _, node := range addObjectsToGraph(graph, targetables) {
		node.SetShape(cgraph.BoxShape)
		node.SetStyle(cgraph.FilledNodeStyle)
		node.SetFillColor("#e5e5e5")
	}
}

func addPoliciesToGraph[T Policy](graph *cgraph.Graph, policies []T) {
	for i, policyNode := range addObjectsToGraph(graph, policies) {
		policyNode.SetShape(cgraph.NoteShape)
		policyNode.SetStyle(cgraph.DashedNodeStyle)
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

func addEdgeToGraph(graph *cgraph.Graph, name string, parent, child Object) {
	p, _ := graph.Node(string(parent.GetURL()))
	c, _ := graph.Node(string(child.GetURL()))
	if p != nil && c != nil {
		graph.CreateEdge(name, p, c)
	}
}

func associateURL[T Object](obj T) (string, T) {
	return obj.GetURL(), obj
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
func (c *collection[T]) Parents(item T) []T {
	var parents []T
	n, err := c.topology.graph.Node(item.GetURL())
	if err != nil {
		return nil
	}
	edge := c.topology.graph.FirstIn(n)
	for {
		if edge == nil {
			break
		}
		_, ok := c.items[edge.Node().Name()]
		if ok {
			parents = append(parents, c.items[edge.Node().Name()])
		}
		edge = c.topology.graph.NextIn(edge)
	}
	return parents
}

// Children returns all children of a given item in the collection.
func (c *collection[T]) Children(item T) []T {
	var children []T
	n, err := c.topology.graph.Node(item.GetURL())
	if err != nil {
		return nil
	}
	edge := c.topology.graph.FirstOut(n)
	for {
		if edge == nil {
			break
		}
		_, ok := c.items[edge.Node().Name()]
		if ok {
			children = append(children, c.items[edge.Node().Name()])
		}
		edge = c.topology.graph.NextOut(edge)
	}
	return children
}

// Paths returns all paths from a source item to a destination item in the collection.
// The order of the elements in the inner slices represents a path from the source to the destination.
func (c *collection[T]) Paths(from, to T) [][]T {
	if &from == nil || &to == nil {
		return nil
	}
	var paths [][]T
	var path []T
	visited := make(map[string]bool)
	c.dfs(from, to, path, &paths, visited)
	return paths
}

// dfs performs a depth-first search to find all paths from a source item to a destination item in the collection.
func (c *collection[T]) dfs(current, to T, path []T, paths *[][]T, visited map[string]bool) {
	currentURL := current.GetURL()
	if visited[currentURL] {
		return
	}
	path = append(path, c.items[currentURL])
	visited[currentURL] = true
	if currentURL == to.GetURL() {
		pathCopy := make([]T, len(path))
		copy(pathCopy, path)
		*paths = append(*paths, pathCopy)
	} else {
		for _, child := range c.Children(current) {
			c.dfs(child, to, path, paths, visited)
		}
	}
	path = path[:len(path)-1]
	visited[currentURL] = false
}
