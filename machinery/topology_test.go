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
	topology, err := NewTopology(
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

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	roots := topology.Targetables().Roots()
	if expected := len(apples); len(roots) != expected {
		t.Errorf("expected %d roots, got %d", expected, len(roots))
	}
	rootLocators := lo.Map(roots, MapTargetableToLocatorFunc)
	for _, apple := range apples {
		if !lo.Contains(rootLocators, apple.GetLocator()) {
			t.Errorf("expected root %s not found", apple.GetLocator())
		}
	}
}

func TestTopologyParents(t *testing.T) {
	apple1 := &Apple{Name: "apple-1"}
	apple2 := &Apple{Name: "apple-2"}
	orange1 := &Orange{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1", "apple-2"}}
	orange2 := &Orange{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-2"}}
	topology, err := NewTopology(
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

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// orange-1
	parents := topology.Targetables().Parents(orange1)
	if expected := 2; len(parents) != expected {
		t.Errorf("expected %d parent, got %d", expected, len(parents))
	}
	parentLocators := lo.Map(parents, MapTargetableToLocatorFunc)
	if !lo.Contains(parentLocators, apple1.GetLocator()) {
		t.Errorf("expected parent %s not found", apple1.GetLocator())
	}
	if !lo.Contains(parentLocators, apple2.GetLocator()) {
		t.Errorf("expected parent %s not found", apple2.GetLocator())
	}
	// orange-2
	parents = topology.Targetables().Parents(orange2)
	if expected := 1; len(parents) != expected {
		t.Errorf("expected %d parent, got %d", expected, len(parents))
	}
	parentLocators = lo.Map(parents, MapTargetableToLocatorFunc)
	if !lo.Contains(parentLocators, apple2.GetLocator()) {
		t.Errorf("expected parent %s not found", apple2.GetLocator())
	}
}

func TestTopologyChildren(t *testing.T) {
	apple1 := &Apple{Name: "apple-1"}
	apple2 := &Apple{Name: "apple-2"}
	orange1 := &Orange{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1", "apple-2"}}
	orange2 := &Orange{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-2"}}
	topology, err := NewTopology(
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

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// apple-1
	children := topology.Targetables().Children(apple1)
	if expected := 1; len(children) != expected {
		t.Errorf("expected %d child, got %d", expected, len(children))
	}
	childLocators := lo.Map(children, MapTargetableToLocatorFunc)
	if !lo.Contains(childLocators, orange1.GetLocator()) {
		t.Errorf("expected child %s not found", orange1.GetLocator())
	}
	// apple-2
	children = topology.Targetables().Children(apple2)
	if expected := 2; len(children) != expected {
		t.Errorf("expected %d child, got %d", expected, len(children))
	}
	childLocators = lo.Map(children, MapTargetableToLocatorFunc)
	if !lo.Contains(childLocators, orange1.GetLocator()) {
		t.Errorf("expected child %s not found", orange1.GetLocator())
	}
	if !lo.Contains(childLocators, orange2.GetLocator()) {
		t.Errorf("expected child %s not found", orange2.GetLocator())
	}
}

func TestTopologyPaths(t *testing.T) {
	apples := []*Apple{{Name: "apple-1"}}
	oranges := []*Orange{
		{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1"}, ChildBananas: []string{"banana-1"}},
		{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-1"}, ChildBananas: []string{"banana-1"}},
	}
	bananas := []*Banana{{Name: "banana-1"}}
	topology, err := NewTopology(
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

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

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
				return strings.Join(lo.Map(expectedPath, MapTargetableToLocatorFunc), "→")
			})
			for _, path := range paths {
				pathString := strings.Join(lo.Map(path, MapTargetableToLocatorFunc), "→")
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
			topology, err := NewTopology(
				WithTargetables(tc.targetables.apples...),
				WithTargetables(tc.targetables.oranges...),
				WithTargetables(tc.targetables.bananas...),
				WithLinks(
					LinkApplesToOranges(tc.targetables.apples),
					LinkOrangesToBananas(tc.targetables.oranges),
				),
				WithPolicies(tc.policies...),
			)

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

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

func TestTopologyWithRuntimeObjects(t *testing.T) {
	objects := []*Info{
		{Name: "info-1", Ref: "apple.example.test:apple-1"},
		{Name: "info-2", Ref: "orange.example.test:my-namespace/orange-1"},
	}
	apples := []*Apple{{Name: "apple-1"}}
	oranges := []*Orange{
		{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1"}},
		{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-1"}},
	}

	topology, err := NewTopology(
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

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	expectedLinks := map[string][]string{
		"apple-1":  {"orange-1", "orange-2"},
		"orange-1": {},
		"orange-2": {},
	}

	links := make(map[string][]string)
	for _, root := range topology.Targetables().Roots() {
		linksFromTargetable(topology, root, links)
	}

	if len(links) != len(expectedLinks) {
		t.Errorf("expected links length to be %v, got %v", len(expectedLinks), len(links))
	}

	for expectedFrom, expectedTos := range expectedLinks {
		tos, ok := links[expectedFrom]
		if !ok {
			t.Errorf("expected root for %v, got none", expectedFrom)
		}
		slices.Sort(expectedTos)
		slices.Sort(tos)
		if !slices.Equal(expectedTos, tos) {
			t.Errorf("expected links from %s to be %v, got %v", expectedFrom, expectedTos, tos)
		}
	}

	SaveToOutputDir(t, topology.ToDot(), "../tests/out", ".dot")
}
func TestTopologyHasLoops(t *testing.T) {
	apples := []*Apple{{Name: "apple-1"}, {Name: "apple-2"}}
	oranges := []*Orange{
		{Name: "orange-1", AppleParents: []string{"apple-1"}},
		{Name: "orange-2", AppleParents: []string{"apple-1", "apple-2"}},
		{Name: "orange-3", AppleParents: []string{"apple-2"}},
	}
	peaches := []*Peach{
		{Name: "peach-1", OrangeParents: []string{"orange-1"}, ChildApples: []string{"apple-1"}},
		{Name: "peach-2", OrangeParents: []string{"orange-1", "orange-2", "orange-3"}},
	}

	lemons := []*Lemon{
		{Name: "lemon-1", PeachParents: []string{"peach-1"}},
	}
	_, err := NewTopology(
		WithTargetables(lemons...),
		WithTargetables(apples...),
		WithTargetables(peaches...),
		WithTargetables(oranges...),
		WithLinks(
			LinkPeachesToLemons(peaches),
			LinkApplesToOranges(apples),
			LinkPeachesToApples(peaches),
			LinkOrangesToPeaches(oranges),
		),
	)
	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if err != nil && !strings.Contains(err.Error(), "loop detected") {
		t.Errorf("Expected loop detection error, got: %s", err.Error())
	}
}

func TestTopologyHasLoopsAndAllowed(t *testing.T) {
	apples := []*Apple{{Name: "apple-1"}, {Name: "apple-2"}}
	oranges := []*Orange{
		{Name: "orange-1", AppleParents: []string{"apple-1"}},
		{Name: "orange-2", AppleParents: []string{"apple-1", "apple-2"}},
		{Name: "orange-3", AppleParents: []string{"apple-2"}},
	}
	peaches := []*Peach{
		{Name: "peach-1", OrangeParents: []string{"orange-1"}, ChildApples: []string{"apple-1"}},
		{Name: "peach-2", OrangeParents: []string{"orange-1", "orange-2", "orange-3"}},
	}

	lemons := []*Lemon{
		{Name: "lemon-1", PeachParents: []string{"peach-1"}},
	}
	topology, err := NewTopology(
		WithTargetables(lemons...),
		WithTargetables(apples...),
		WithTargetables(peaches...),
		WithTargetables(oranges...),
		WithLinks(
			LinkPeachesToLemons(peaches),
			LinkApplesToOranges(apples),
			LinkPeachesToApples(peaches),
			LinkOrangesToPeaches(oranges),
		),
		AllowLoops(),
	)
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}

	if topology == nil {
		t.Errorf("Expected topology, got nil")
	}

}

func TestTopologyHasNoLoops(t *testing.T) {
	apples := []*Apple{{Name: "apple-1"}, {Name: "apple-2"}, {Name: "apple-3"}}
	oranges := []*Orange{
		{Name: "orange-1", AppleParents: []string{"apple-1"}},
		{Name: "orange-2", AppleParents: []string{"apple-2"}},
		{Name: "orange-3", AppleParents: []string{"apple-3", "apple-1"}},
		{Name: "orange-4", AppleParents: []string{"apple-3"}},
	}
	peaches := []*Peach{
		{Name: "peach-1", OrangeParents: []string{"orange-1"}},
		{Name: "peach-2", OrangeParents: []string{"orange-1", "orange-3", "orange-4"}},
	}

	lemons := []*Lemon{
		{Name: "lemon-1", PeachParents: []string{"peach-1"}},
	}
	_, err := NewTopology(
		WithTargetables(lemons...),
		WithTargetables(apples...),
		WithTargetables(peaches...),
		WithTargetables(oranges...),
		WithLinks(
			LinkPeachesToLemons(peaches),
			LinkApplesToOranges(apples),
			LinkPeachesToApples(peaches),
			LinkOrangesToPeaches(oranges),
		),
	)
	if err != nil {
		t.Errorf("Expected no error, got: %s", err.Error())
	}
}

func TestTopologyAll(t *testing.T) {
	objects := []*Info{
		{Name: "info-1", Ref: "apple.example.test:apple-1"},
		{Name: "info-2", Ref: "orange.example.test:my-namespace/orange-1"},
	}
	apples := []*Apple{{Name: "apple-1"}}
	oranges := []*Orange{
		{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1"}},
		{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-1"}},
	}
	policies := []Policy{
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
	}

	topology, err := NewTopology(
		WithObjects(objects...),
		WithTargetables(apples...),
		WithTargetables(oranges...),
		WithPolicies(policies...),
		WithLinks(
			LinkApplesToOranges(apples),
			LinkInfoFrom("Apple", lo.Map(apples, AsObject[*Apple])),
			LinkInfoFrom("Orange", lo.Map(oranges, AsObject[*Orange])),
		),
	)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	SaveToOutputDir(t, topology.ToDot(), "../tests/out", ".dot")

	expectedLinks := map[string][]string{
		"policy-1": {"apple-1"},
		"policy-2": {"orange-1"},
		"apple-1":  {"orange-1", "orange-2", "info-1"},
		"orange-1": {"info-2"},
		"orange-2": {},
		"info-1":   {},
		"info-2":   {},
	}

	links := make(map[string][]string)
	for _, root := range topology.All().Roots() {
		linksFromAll(topology, root, links)
	}

	if len(links) != len(expectedLinks) {
		t.Errorf("expected links length to be %v, got %v", len(expectedLinks), len(links))
	}

	for expectedFrom, expectedTos := range expectedLinks {
		tos, ok := links[expectedFrom]
		if !ok {
			t.Errorf("expected root for %v, got none", expectedFrom)
		}
		slices.Sort(expectedTos)
		slices.Sort(tos)
		if !slices.Equal(expectedTos, tos) {
			t.Errorf("expected links from %s to be %v, got %v", expectedFrom, expectedTos, tos)
		}
	}
}

func TestTopologyAllPaths(t *testing.T) {
	objects := []*Info{
		{Name: "info-1", Ref: "apple.example.test:apple-1"},
		{Name: "info-2", Ref: "orange.example.test:my-namespace/orange-1"},
	}
	apples := []*Apple{{Name: "apple-1"}}
	oranges := []*Orange{
		{Name: "orange-1", Namespace: "my-namespace", AppleParents: []string{"apple-1"}},
		{Name: "orange-2", Namespace: "my-namespace", AppleParents: []string{"apple-1"}},
	}
	policies := []Policy{
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
	}

	topology, err := NewTopology(
		WithObjects(objects...),
		WithTargetables(apples...),
		WithTargetables(oranges...),
		WithPolicies(policies...),
		WithLinks(
			LinkApplesToOranges(apples),
			LinkInfoFrom("Apple", lo.Map(apples, AsObject[*Apple])),
			LinkInfoFrom("Orange", lo.Map(oranges, AsObject[*Orange])),
		),
	)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	SaveToOutputDir(t, topology.ToDot(), "../tests/out", ".dot")

	testCases := []struct {
		name          string
		from          Object
		to            Object
		expectedPaths [][]Object
	}{
		{
			name: "policy to targetable",
			from: policies[0],
			to:   apples[0],
			expectedPaths: [][]Object{
				{policies[0], apples[0]},
			},
		},
		{
			name: "targetable to targetable",
			from: apples[0],
			to:   oranges[0],
			expectedPaths: [][]Object{
				{apples[0], oranges[0]},
			},
		},
		{
			name: "targetable to object",
			from: oranges[0],
			to:   objects[1],
			expectedPaths: [][]Object{
				{oranges[0], objects[1]},
			},
		},
		{
			name: "policy to object",
			from: policies[0],
			to:   objects[1],
			expectedPaths: [][]Object{
				{policies[0], apples[0], oranges[0], objects[1]},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths := topology.All().Paths(tc.from, tc.to)
			if len(paths) != len(tc.expectedPaths) {
				t.Errorf("expected %d paths, got %d", len(tc.expectedPaths), len(paths))
			}
			expectedPaths := lo.Map(tc.expectedPaths, func(expectedPath []Object, _ int) string {
				return strings.Join(lo.Map(expectedPath, MapObjectToLocatorFunc), "→")
			})
			for _, path := range paths {
				pathString := strings.Join(lo.Map(path, MapObjectToLocatorFunc), "→")
				if !lo.Contains(expectedPaths, pathString) {
					t.Errorf("expected path %v not found", pathString)
				}
			}
		})
	}
}
