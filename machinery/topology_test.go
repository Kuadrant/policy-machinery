//go:build unit

package machinery

import (
	"slices"
	"strings"
	"testing"

	"github.com/samber/lo"
)

func TestTopologyRoots(t *testing.T) {
	apples := []*Apple{
		{Name: "apple-1"},
		{Name: "apple-2"},
	}
	topology := NewTopology(
		WithTargetables(apples...),
		WithTargetables(&Orange{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1"}}),
		WithLinks(LinkApplesToOranges(apples)),
		WithPolicies(
			buildFruitPolicy(func(policy *FruitPolicy) {
				policy.Name = "policy-1"
				policy.Spec.TargetRef = FruitPolicyTargetReference{
					Group: TestGroupName,
					Kind:  "Apple",
					Name:  "apple-2",
				}
			}),
			buildFruitPolicy(func(policy *FruitPolicy) {
				policy.Name = "policy-2"
				policy.Spec.TargetRef = FruitPolicyTargetReference{
					Group: TestGroupName,
					Kind:  "Orange",
					Name:  "orange-1",
				}
			}),
		),
	)
	roots := topology.Targetables().Roots()
	if expected := len(apples); len(roots) != expected {
		t.Errorf("expected %d roots, got %d", expected, len(roots))
	}
	rootURLs := lo.Map(roots, MapTargetableToURLFunc)
	for _, apple := range apples {
		if !lo.Contains(rootURLs, apple.GetURL()) {
			t.Errorf("expected root %s not found", apple.GetURL())
		}
	}
}

func TestTopologyParents(t *testing.T) {
	apple1 := &Apple{Name: "apple-1"}
	apple2 := &Apple{Name: "apple-2"}
	orange1 := &Orange{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1", "apple-2"}}
	orange2 := &Orange{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-2"}}
	topology := NewTopology(
		WithTargetables(apple1, apple2),
		WithTargetables(orange1, orange2),
		WithLinks(LinkApplesToOranges([]*Apple{apple1, apple2})),
		WithPolicies(
			buildFruitPolicy(func(policy *FruitPolicy) {
				policy.Name = "policy-2"
				policy.Spec.TargetRef = FruitPolicyTargetReference{
					Group: TestGroupName,
					Kind:  "Orange",
					Name:  "orange-1",
				}
			}),
		),
	)
	// orange-1
	parents := topology.Targetables().Parents(orange1)
	if expected := 2; len(parents) != expected {
		t.Errorf("expected %d parent, got %d", expected, len(parents))
	}
	parentURLs := lo.Map(parents, MapTargetableToURLFunc)
	if !lo.Contains(parentURLs, apple1.GetURL()) {
		t.Errorf("expected parent %s not found", apple1.GetURL())
	}
	if !lo.Contains(parentURLs, apple2.GetURL()) {
		t.Errorf("expected parent %s not found", apple2.GetURL())
	}
	// orange-2
	parents = topology.Targetables().Parents(orange2)
	if expected := 1; len(parents) != expected {
		t.Errorf("expected %d parent, got %d", expected, len(parents))
	}
	parentURLs = lo.Map(parents, MapTargetableToURLFunc)
	if !lo.Contains(parentURLs, apple2.GetURL()) {
		t.Errorf("expected parent %s not found", apple2.GetURL())
	}
}

func TestTopologyChildren(t *testing.T) {
	apple1 := &Apple{Name: "apple-1"}
	apple2 := &Apple{Name: "apple-2"}
	orange1 := &Orange{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1", "apple-2"}}
	orange2 := &Orange{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-2"}}
	topology := NewTopology(
		WithTargetables(apple1, apple2),
		WithTargetables(orange1, orange2),
		WithLinks(LinkApplesToOranges([]*Apple{apple1, apple2})),
		WithPolicies(
			buildFruitPolicy(func(policy *FruitPolicy) {
				policy.Name = "policy-2"
				policy.Spec.TargetRef = FruitPolicyTargetReference{
					Group: TestGroupName,
					Kind:  "Orange",
					Name:  "orange-1",
				}
			}),
		),
	)
	// apple-1
	children := topology.Targetables().Children(apple1)
	if expected := 1; len(children) != expected {
		t.Errorf("expected %d child, got %d", expected, len(children))
	}
	childURLs := lo.Map(children, MapTargetableToURLFunc)
	if !lo.Contains(childURLs, orange1.GetURL()) {
		t.Errorf("expected child %s not found", orange1.GetURL())
	}
	// apple-2
	children = topology.Targetables().Children(apple2)
	if expected := 2; len(children) != expected {
		t.Errorf("expected %d child, got %d", expected, len(children))
	}
	childURLs = lo.Map(children, MapTargetableToURLFunc)
	if !lo.Contains(childURLs, orange1.GetURL()) {
		t.Errorf("expected child %s not found", orange1.GetURL())
	}
	if !lo.Contains(childURLs, orange2.GetURL()) {
		t.Errorf("expected child %s not found", orange2.GetURL())
	}
}

func TestTopologyPaths(t *testing.T) {
	apples := []*Apple{{Name: "apple-1"}}
	oranges := []*Orange{
		{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1"}, ChildBananas: []string{"banana-1"}},
		{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-1"}, ChildBananas: []string{"banana-1"}},
	}
	bananas := []*Banana{{Name: "banana-1"}}
	topology := NewTopology(
		WithTargetables(apples...),
		WithTargetables(oranges...),
		WithTargetables(bananas...),
		WithLinks(
			LinkApplesToOranges(apples),
			LinkOrangesToBananas(oranges),
		),
		WithPolicies(
			buildFruitPolicy(func(policy *FruitPolicy) {
				policy.Name = "policy-2"
				policy.Spec.TargetRef = FruitPolicyTargetReference{
					Group: TestGroupName,
					Kind:  "Orange",
					Name:  "orange-1",
				}
			}),
		),
	)
	testCases := []struct {
		name          string
		from          Targetable
		to            Targetable
		expectedPaths [][]Targetable
	}{
		{
			name: "single path",
			from: oranges[0],
			to:   bananas[0],
			expectedPaths: [][]Targetable{
				{oranges[0], bananas[0]},
			},
		},
		{
			name: "multiple paths",
			from: apples[0],
			to:   bananas[0],
			expectedPaths: [][]Targetable{
				{apples[0], oranges[0], bananas[0]},
				{apples[0], oranges[1], bananas[0]},
			},
		},
		{
			name: "trivial path",
			from: apples[0],
			to:   apples[0],
			expectedPaths: [][]Targetable{
				{apples[0]},
			},
		},
		{
			name:          "no path",
			from:          bananas[0],
			to:            apples[0],
			expectedPaths: [][]Targetable{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths := topology.Targetables().Paths(tc.from, tc.to)
			if len(paths) != len(tc.expectedPaths) {
				t.Errorf("expected %d paths, got %d", len(tc.expectedPaths), len(paths))
			}
			expectedPaths := lo.Map(tc.expectedPaths, func(expectedPath []Targetable, _ int) string {
				return strings.Join(lo.Map(expectedPath, MapTargetableToURLFunc), "→")
			})
			for _, path := range paths {
				pathString := strings.Join(lo.Map(path, MapTargetableToURLFunc), "→")
				if !lo.Contains(expectedPaths, pathString) {
					t.Errorf("expected path %v not found", pathString)
				}
			}
		})
	}
}

type fruits struct {
	apples  []*Apple
	oranges []*Orange
	bananas []*Banana
}

func TestFruitTopology(t *testing.T) {
	testCases := []struct {
		name          string
		targetables   fruits
		policies      []Policy
		expectedLinks map[string][]string
	}{
		{
			name: "empty",
		},
		{
			name: "single node",
			targetables: fruits{
				apples: []*Apple{{Name: "my-apple"}},
			},
		},
		{
			name: "multiple gvk",
			targetables: fruits{
				apples:  []*Apple{{Name: "my-apple"}},
				oranges: []*Orange{{Name: "my-orange", Namespace: "my-namespace", AppleParents: []string{"my-apple"}}},
			},
			policies: []Policy{buildFruitPolicy()},
			expectedLinks: map[string][]string{
				"my-apple": {"my-orange"},
			},
		},
		{
			name: "complex topology",
			targetables: fruits{
				apples: []*Apple{{Name: "apple-1"}, {Name: "apple-2"}},
				oranges: []*Orange{
					{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1", "apple-2"}, ChildBananas: []string{"banana-1", "banana-2"}},
					{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-2"}, ChildBananas: []string{"banana-2", "banana-3"}},
				},
				bananas: []*Banana{{Name: "banana-1"}, {Name: "banana-2"}, {Name: "banana-3"}},
			},
			policies: []Policy{
				buildFruitPolicy(func(policy *FruitPolicy) {
					policy.Name = "policy-1"
					policy.Spec.TargetRef.Kind = "Apple"
					policy.Spec.TargetRef.Name = "apple-1"
				}),
				buildFruitPolicy(func(policy *FruitPolicy) {
					policy.Name = "policy-2"
					policy.Spec.TargetRef.Kind = "Orange"
					policy.Spec.TargetRef.Name = "orange-2"
				}),
			},
			expectedLinks: map[string][]string{
				"apple-1":  {"orange-1"},
				"apple-2":  {"orange-1", "orange-2"},
				"orange-1": {"banana-1", "banana-2"},
				"orange-2": {"banana-2", "banana-3"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			topology := NewTopology(
				WithTargetables(tc.targetables.apples...),
				WithTargetables(tc.targetables.oranges...),
				WithTargetables(tc.targetables.bananas...),
				WithLinks(
					LinkApplesToOranges(tc.targetables.apples),
					LinkOrangesToBananas(tc.targetables.oranges),
				),
				WithPolicies(tc.policies...),
			)

			links := make(map[string][]string)
			for _, root := range topology.Targetables().Roots() {
				linksFromTargetable(topology, root, links)
			}
			for from, tos := range links {
				expectedTos := tc.expectedLinks[from]
				slices.Sort(expectedTos)
				slices.Sort(tos)
				if !slices.Equal(expectedTos, tos) {
					t.Errorf("expected links from %s to be %v, got %v", from, expectedTos, tos)
				}
			}

			SaveToOutputDir(t, topology.ToDot(), "../tests/out", ".dot")
		})
	}
}

func TestTopologyWithGenericObjects(t *testing.T) {
	objects := []*Info{
		{Name: "info-1", Ref: "apple.example.test:apple-1"},
		{Name: "info-2", Ref: "orange.example.test:my-namespace/orange-1"},
	}
	apples := []*Apple{{Name: "apple-1"}}
	oranges := []*Orange{
		{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1"}},
		{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-1"}},
	}

	topology := NewTopology(
		WithObjects(objects...),
		WithTargetables(apples...),
		WithTargetables(oranges...),
		WithPolicies(
			buildFruitPolicy(func(policy *FruitPolicy) {
				policy.Name = "policy-1"
				policy.Spec.TargetRef.Kind = "Apple"
				policy.Spec.TargetRef.Name = "apple-1"
			}),
			buildFruitPolicy(func(policy *FruitPolicy) {
				policy.Name = "policy-2"
				policy.Spec.TargetRef.Kind = "Orange"
				policy.Spec.TargetRef.Name = "orange-1"
			}),
		),
		WithLinks(
			LinkApplesToOranges(apples),
			LinkInfoFrom("Apple", lo.Map(apples, AsObject[*Apple])),
			LinkInfoFrom("Orange", lo.Map(oranges, AsObject[*Orange])),
		),
	)

	expectedLinks := map[string][]string{
		"apple-1": {"orange-1", "orange-2"},
		"info-1":  {"apple-1"},
		"info-2":  {"orange-1"},
	}

	links := make(map[string][]string)
	for _, root := range topology.Targetables().Roots() {
		linksFromTargetable(topology, root, links)
	}
	for from, tos := range links {
		expectedTos := expectedLinks[from]
		slices.Sort(expectedTos)
		slices.Sort(tos)
		if !slices.Equal(expectedTos, tos) {
			t.Errorf("expected links from %s to be %v, got %v", from, expectedTos, tos)
		}
	}

	SaveToOutputDir(t, topology.ToDot(), "../tests/out", ".dot")
}
