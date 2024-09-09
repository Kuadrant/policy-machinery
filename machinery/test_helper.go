//go:build unit || integration

package machinery

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

// SaveToOutputDir saves the output of a test case to a file in the output directory.
func SaveToOutputDir(t *testing.T, out string, outDir, ext string) {
	file, err := os.Create(fmt.Sprintf("%s/%s%s", outDir, strings.ReplaceAll(t.Name(), "/", "__"), ext))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, err = file.WriteString(out)
	if err != nil {
		t.Fatal(err)
	}
}

func linksFromTargetable(topology *Topology, targetable Targetable, edges map[string][]string) {
	if _, ok := edges[targetable.GetName()]; ok {
		return
	}
	children := topology.Targetables().Children(targetable)
	edges[targetable.GetName()] = lo.Map(children, func(child Targetable, _ int) string { return child.GetName() })
	for _, child := range children {
		linksFromTargetable(topology, child, edges)
	}
}

const TestGroupName = "example.test"

type Apple struct {
	Name string

	policies []Policy
}

var _ Targetable = &Apple{}

func (a *Apple) GetName() string {
	return a.Name
}

func (a *Apple) GetNamespace() string {
	return ""
}

func (a *Apple) GetLocator() string {
	return LocatorFromObject(a)
}

func (a *Apple) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   TestGroupName,
		Version: "v1",
		Kind:    "Apple",
	}
}

func (a *Apple) SetGroupVersionKind(schema.GroupVersionKind) {}

func (a *Apple) Policies() []Policy {
	return a.policies
}

func (a *Apple) SetPolicies(policies []Policy) {
	a.policies = policies
}

type Orange struct {
	Name         string
	Namespace    string
	AppleParents []string
	ChildBananas []string

	policies []Policy
}

var _ Targetable = &Orange{}

func (o *Orange) GetName() string {
	return o.Name
}

func (o *Orange) GetNamespace() string {
	return o.Namespace
}

func (o *Orange) GetLocator() string {
	return LocatorFromObject(o)
}

func (o *Orange) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   TestGroupName,
		Version: "v1beta1",
		Kind:    "Orange",
	}
}

func (o *Orange) SetGroupVersionKind(schema.GroupVersionKind) {}

func (o *Orange) Policies() []Policy {
	return o.policies
}

func (o *Orange) SetPolicies(policies []Policy) {
	o.policies = policies
}

type Banana struct {
	Name string
}

var _ Targetable = &Banana{}

func (b *Banana) GetName() string {
	return b.Name
}

func (b *Banana) GetNamespace() string {
	return ""
}

func (b *Banana) GetLocator() string {
	return LocatorFromObject(b)
}

func (b *Banana) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   TestGroupName,
		Version: "v1beta1",
		Kind:    "Banana",
	}
}

func (b *Banana) SetGroupVersionKind(schema.GroupVersionKind) {}

func (b *Banana) Policies() []Policy {
	return nil
}

func (b *Banana) SetPolicies(policies []Policy) {}

func LinkApplesToOranges(apples []*Apple) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: TestGroupName, Kind: "Apple"},
		To:   schema.GroupKind{Group: TestGroupName, Kind: "Orange"},
		Func: func(child Object) []Object {
			orange := child.(*Orange)
			return lo.FilterMap(apples, func(apple *Apple, _ int) (Object, bool) {
				return apple, lo.Contains(orange.AppleParents, apple.Name)
			})
		},
	}
}

func LinkOrangesToBananas(oranges []*Orange) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: TestGroupName, Kind: "Orange"},
		To:   schema.GroupKind{Group: TestGroupName, Kind: "Banana"},
		Func: func(child Object) []Object {
			banana := child.(*Banana)
			return lo.FilterMap(oranges, func(orange *Orange, _ int) (Object, bool) {
				return orange, lo.Contains(orange.ChildBananas, banana.Name)
			})
		},
	}
}

type Info struct {
	Name string
	Ref  string
}

var _ Object = &Info{}

func (i *Info) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   TestGroupName,
		Version: "v1",
		Kind:    "Info",
	}
}

func (i *Info) SetGroupVersionKind(schema.GroupVersionKind) {}

func (i *Info) GetNamespace() string {
	return ""
}

func (i *Info) GetName() string {
	return i.Name
}

func (i *Info) GetLocator() string {
	return LocatorFromObject(i)
}

func LinkInfoFrom(kind string, objects []Object) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: TestGroupName, Kind: kind},
		To:   schema.GroupKind{Group: TestGroupName, Kind: "Info"},
		Func: func(child Object) []Object {
			info := child.(*Info)
			return lo.Filter(objects, func(obj Object, _ int) bool {
				return obj.GetLocator() == info.Ref
			})
		},
	}
}

type FruitPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FruitPolicySpec `json:"spec"`
}

type FruitPolicySpec struct {
	TargetRef FruitPolicyTargetReference `json:"targetRef"`
}

var _ Policy = &FruitPolicy{}

func (p *FruitPolicy) GetLocator() string {
	return LocatorFromObject(p)
}

func (p *FruitPolicy) GetTargetRefs() []PolicyTargetReference {
	var namespace *string
	group := p.Spec.TargetRef.Group
	kind := p.Spec.TargetRef.Kind
	if group == TestGroupName && kind == "Orange" {
		namespace = ptr.To(ptr.Deref(p.Spec.TargetRef.Namespace, p.Namespace))
	}
	return []PolicyTargetReference{
		FruitPolicyTargetReference{
			Group:     group,
			Kind:      kind,
			Name:      p.Spec.TargetRef.Name,
			Namespace: namespace,
		},
	}
}

func (p *FruitPolicy) GetMergeStrategy() MergeStrategy {
	return DefaultMergeStrategy
}

func (p *FruitPolicy) Merge(policy Policy) Policy {
	return &FruitPolicy{
		Spec: p.Spec,
	}
}

type FruitPolicyTargetReference struct {
	Group     string
	Kind      string
	Name      string
	Namespace *string
}

var _ PolicyTargetReference = FruitPolicyTargetReference{}

func (t FruitPolicyTargetReference) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group: t.Group,
		Kind:  t.Kind,
	}
}

func (t FruitPolicyTargetReference) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	t.Group = gvk.Group
	t.Kind = gvk.Kind
}

func (t FruitPolicyTargetReference) GetLocator() string {
	return LocatorFromObject(t)
}

func (t FruitPolicyTargetReference) GetNamespace() string {
	return ptr.Deref(t.Namespace, "")
}

func (t FruitPolicyTargetReference) GetName() string {
	return t.Name
}

func buildFruitPolicy(f ...func(*FruitPolicy)) *FruitPolicy {
	p := &FruitPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test/v1",
			Kind:       "FruitPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-policy",
			Namespace: "my-namespace",
		},
		Spec: FruitPolicySpec{
			TargetRef: FruitPolicyTargetReference{
				Group: TestGroupName,
				Kind:  "Orange",
				Name:  "my-orange",
			},
		},
	}
	for _, fn := range f {
		fn(p)
	}
	return p
}

type Peach struct {
	Name          string
	Namespace     string
	OrangeParents []string
	ChildApples   []string

	policies []Policy
}

var _ Targetable = &Peach{}

func (o *Peach) GetName() string {
	return o.Name
}

func (o *Peach) GetNamespace() string {
	return o.Namespace
}

func (o *Peach) GetLocator() string {
	return LocatorFromObject(o)
}

func (o *Peach) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   TestGroupName,
		Version: "v1beta1",
		Kind:    "Peach",
	}
}

func (o *Peach) SetGroupVersionKind(schema.GroupVersionKind) {}

func (o *Peach) Policies() []Policy {
	return o.policies
}

func (o *Peach) SetPolicies(policies []Policy) {
	o.policies = policies
}

func LinkOrangesToPeaches(oranges []*Orange) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: TestGroupName, Kind: "Orange"},
		To:   schema.GroupKind{Group: TestGroupName, Kind: "Peach"},
		Func: func(child Object) []Object {
			peach := child.(*Peach)
			return lo.FilterMap(oranges, func(orange *Orange, _ int) (Object, bool) {
				return orange, lo.Contains(peach.OrangeParents, orange.Name)
			})
		},
	}
}

func LinkPeachesToApples(peaches []*Peach) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: TestGroupName, Kind: "Peach"},
		To:   schema.GroupKind{Group: TestGroupName, Kind: "Apple"},
		Func: func(child Object) []Object {
			apple := child.(*Apple)
			return lo.FilterMap(peaches, func(peach *Peach, _ int) (Object, bool) {
				return peach, lo.Contains(peach.ChildApples, apple.Name)
			})
		},
	}
}

type Lemon struct {
	Name         string
	Namespace    string
	PeachParents []string

	policies []Policy
}

var _ Targetable = &Lemon{}

func (o *Lemon) GetName() string {
	return o.Name
}

func (o *Lemon) GetNamespace() string {
	return o.Namespace
}

func (o *Lemon) GetLocator() string {
	return LocatorFromObject(o)
}

func (o *Lemon) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   TestGroupName,
		Version: "v1beta1",
		Kind:    "Lemon",
	}
}

func (o *Lemon) SetGroupVersionKind(schema.GroupVersionKind) {}

func (o *Lemon) Policies() []Policy {
	return o.policies
}

func (o *Lemon) SetPolicies(policies []Policy) {
	o.policies = policies
}

func LinkPeachesToLemons(peaches []*Peach) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: TestGroupName, Kind: "Peach"},
		To:   schema.GroupKind{Group: TestGroupName, Kind: "Lemon"},
		Func: func(child Object) []Object {
			lemon := child.(*Lemon)
			return lo.FilterMap(peaches, func(peach *Peach, _ int) (Object, bool) {
				return peach, lo.Contains(lemon.PeachParents, peach.Name)
			})
		},
	}
}
