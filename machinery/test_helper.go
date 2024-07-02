//go:build unit || integration

package machinery

import (
	"bytes"
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
func SaveToOutputDir(t *testing.T, out *bytes.Buffer, outDir, ext string) {
	file, err := os.Create(fmt.Sprintf("%s/%s%s", outDir, strings.ReplaceAll(t.Name(), "/", "__"), ext))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, err = file.Write(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
}

func linksFromNode(topology *Topology, node Targetable, edges map[string][]string) {
	if _, ok := edges[node.GetName()]; ok {
		return
	}
	children := topology.Children(node)
	edges[node.GetName()] = lo.Map(children, func(child Targetable, _ int) string { return child.GetName() })
	for _, child := range children {
		linksFromNode(topology, child, edges)
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

func (a *Apple) GetURL() string {
	return UrlFromObject(a)
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

func (o *Orange) GetURL() string {
	return UrlFromObject(o)
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

func (b *Banana) GetURL() string {
	return UrlFromObject(b)
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
		Func: func(child Targetable) []Targetable {
			orange := child.(*Orange)
			return lo.FilterMap(apples, func(apple *Apple, _ int) (Targetable, bool) {
				return apple, lo.Contains(orange.AppleParents, apple.Name)
			})
		},
	}
}

func LinkOrangesToBananas(oranges []*Orange) LinkFunc {
	return LinkFunc{
		From: schema.GroupKind{Group: TestGroupName, Kind: "Orange"},
		To:   schema.GroupKind{Group: TestGroupName, Kind: "Banana"},
		Func: func(child Targetable) []Targetable {
			banana := child.(*Banana)
			return lo.FilterMap(oranges, func(orange *Orange, _ int) (Targetable, bool) {
				return orange, lo.Contains(orange.ChildBananas, banana.Name)
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

func (p *FruitPolicy) GetURL() string {
	return UrlFromObject(p)
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

func (t FruitPolicyTargetReference) GetURL() string {
	return UrlFromObject(t)
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
