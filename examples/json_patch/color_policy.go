package json_patch

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapi "sigs.k8s.io/gateway-api/apis/v1alpha2"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/kuadrant/policy-machinery/machinery"
)

type ColorPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ColorSpec `json:"spec"`
}

var _ machinery.Policy = &ColorPolicy{}

func (p *ColorPolicy) GetURL() string {
	return machinery.UrlFromObject(p)
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
	return func(parent, child machinery.Policy) machinery.Policy {
		colorParent, okSource := parent.(*ColorPolicy)
		colorChild, okTarget := child.(*ColorPolicy)

		if !okSource || !okTarget {
			return nil
		}

		parentSpecJSON, err := json.Marshal(colorParent.Spec.Proper())
		if err != nil {
			return nil
		}

		childSpecJSON, err := json.Marshal(colorChild.Spec.Proper())
		if err != nil {
			return nil
		}

		var resultSpecJSON []byte

		if overrides := colorParent.Spec.Overrides; overrides != nil {
			resultSpecJSON, err = jsonpatch.MergePatch(childSpecJSON, parentSpecJSON)
		} else {
			resultSpecJSON, err = jsonpatch.MergePatch(parentSpecJSON, childSpecJSON)
		}

		if err != nil {
			return nil
		}

		result := ColorSpecProper{}
		if err := json.Unmarshal(resultSpecJSON, &result); err != nil {
			return nil
		}

		return &ColorPolicy{
			TypeMeta:   colorChild.TypeMeta,
			ObjectMeta: colorChild.ObjectMeta,
			Spec: ColorSpec{
				TargetRef:       colorChild.Spec.TargetRef,
				ColorSpecProper: result,
			},
		}
	}
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

	Defaults  *ColorSpecProper `json:"defaults,omitempty"`
	Overrides *ColorSpecProper `json:"overrides,omitempty"`

	ColorSpecProper `json:""`
}

func (s *ColorSpec) Proper() *ColorSpecProper {
	if s.Defaults != nil {
		return s.Defaults
	}
	if s.Overrides != nil {
		return s.Overrides
	}
	return &s.ColorSpecProper
}

func (s *ColorSpec) DeepCopy() *ColorSpec {
	rules := make(map[string]ColorValue, len(s.Proper().Rules))
	for k, v := range s.Proper().Rules {
		rules[k] = v
	}
	return &ColorSpec{
		TargetRef: s.TargetRef,
		ColorSpecProper: ColorSpecProper{
			Rules: rules,
		},
	}
}

type ColorSpecProper struct {
	Rules map[string]ColorValue `json:"rules,omitempty"`
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
