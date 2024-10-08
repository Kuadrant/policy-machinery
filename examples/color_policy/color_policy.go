package color_policy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapi "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/policy-machinery/machinery"
)

type ColorPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ColorSpec `json:"spec"`
}

var _ machinery.Policy = &ColorPolicy{}

func (p *ColorPolicy) GetLocator() string {
	return machinery.LocatorFromObject(p)
}

func (p *ColorPolicy) GetTargetRefs() []machinery.PolicyTargetReference {
	return []machinery.PolicyTargetReference{
		machinery.LocalPolicyTargetReferenceWithSectionName{
			LocalPolicyTargetReferenceWithSectionName: p.Spec.TargetRef,
			PolicyNamespace: p.Namespace,
		},
	}
}

func (p *ColorPolicy) GetMergeStrategy() machinery.MergeStrategy {
	if spec := p.Spec.Defaults; spec != nil {
		return defaultsMergeStrategy(spec.Strategy)
	}
	if spec := p.Spec.Overrides; spec != nil {
		return overridesMergeStrategy(spec.Strategy)
	}
	return defaultsMergeStrategy(AtomicMergeStrategy)
}

func (p *ColorPolicy) Merge(policy machinery.Policy) machinery.Policy {
	source := policy.(*ColorPolicy)
	return source.GetMergeStrategy()(source, p)
}

func (p *ColorPolicy) DeepCopy() *ColorPolicy {
	spec := p.Spec.DeepCopy()
	return &ColorPolicy{
		TypeMeta:   p.TypeMeta,
		ObjectMeta: p.ObjectMeta,
		Spec:       *spec,
	}
}

type ColorSpec struct {
	TargetRef gwapi.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`

	Defaults  *MergeableColorSpec `json:"defaults,omitempty"`
	Overrides *MergeableColorSpec `json:"overrides,omitempty"`

	ColorSpecProper `json:""`
}

func (s *ColorSpec) Proper() *ColorSpecProper {
	if s.Defaults != nil {
		return &s.Defaults.ColorSpecProper
	}
	if s.Overrides != nil {
		return &s.Overrides.ColorSpecProper
	}
	return &s.ColorSpecProper
}

func (s *ColorSpec) DeepCopy() *ColorSpec {
	rules := make([]ColorRule, len(s.Proper().Rules))
	copy(rules, s.Proper().Rules)
	return &ColorSpec{
		TargetRef: s.TargetRef,
		ColorSpecProper: ColorSpecProper{
			Rules: rules,
		},
	}
}

type MergeableColorSpec struct {
	Strategy string `json:"strategy"`

	ColorSpecProper `json:""`
}

type ColorSpecProper struct {
	Rules []ColorRule `json:"rules,omitempty"`
}

type ColorRule struct {
	Id    string     `json:"id"`
	Color ColorValue `json:"color"`
}

type ColorValue string

const (
	Black  ColorValue = "black"
	Blue   ColorValue = "blue"
	Green  ColorValue = "green"
	Orange ColorValue = "orange"
	Purple ColorValue = "purple"
	Red    ColorValue = "red"
	White  ColorValue = "white"
	Yellow ColorValue = "yellow"
)
