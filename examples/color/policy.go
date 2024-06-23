package color

import (
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapi "sigs.k8s.io/gateway-api/apis/v1alpha2"

	machinery "github.com/guicassolato/policy-machinery/machinery"
)

type ColorPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ColorSpec `json:"spec"`
}

var _ machinery.Policy = &ColorPolicy{}

func (p *ColorPolicy) GetTargetRefs() []machinery.PolicyTargetReference {
	return []machinery.PolicyTargetReference{machinery.LocalPolicyTargetReference{LocalPolicyTargetReference: p.Spec.TargetRef, PolicyNamespace: p.Namespace}}
}

func (p *ColorPolicy) GetSpec() machinery.PolicySpec {
	return &p.Spec
}

func (p *ColorPolicy) Merge(policy machinery.Policy, strategy machinery.MergeStrategy) machinery.Policy {
	other := policy.(*ColorPolicy)
	mergedSpec := p.Spec.Merge(&other.Spec, strategy).(*ColorSpec)
	return &ColorPolicy{
		Spec: *mergedSpec,
	}
}

type ColorSpec struct {
	TargetRef gwapi.LocalPolicyTargetReference `json:"targetRef"`
	Rules     []ColorRule                      `json:"rules"`
}

var _ machinery.PolicySpec = &ColorSpec{}

func (s *ColorSpec) SetRules(rules []machinery.Rule) {
	s.Rules = lo.Map(rules, toColorRule)
}

func (s *ColorSpec) GetRules() []machinery.Rule {
	return lo.Map(s.Rules, toCommonRule)
}

func (s *ColorSpec) DeepCopy() machinery.PolicySpec {
	rules := make([]ColorRule, len(s.Rules))
	copy(rules, s.Rules)
	return &ColorSpec{
		TargetRef: s.TargetRef,
		Rules:     rules,
	}
}

func (s *ColorSpec) Merge(spec machinery.PolicySpec, strategy machinery.MergeStrategy) machinery.PolicySpec {
	mergedSpec := machinery.Merge(spec, s, strategy).(*ColorSpec)
	newSpec := s.DeepCopy()
	newSpec.SetRules(mergedSpec.GetRules())
	return newSpec
}

type ColorRule struct {
	Id    string     `json:"id"`
	Color ColorValue `json:"color"`
}

var _ machinery.Rule = &ColorRule{}

func (r *ColorRule) GetId() machinery.RuleId {
	return machinery.RuleId(r.Id)
}

type ColorValue int

const (
	Red ColorValue = iota
	Blue
	Green
	Yellow
)

func toColorRule(rule machinery.Rule, _ int) ColorRule {
	cr := rule.(*ColorRule)
	return *cr
}

func toCommonRule(rule ColorRule, _ int) machinery.Rule {
	return &rule
}
