package machinery

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// Policy targets objects and contains a PolicySpec that can be merged with another PolicySpec based on a
// given MergeStrategy.
type Policy interface {
	Object

	GetTargetRefs() []PolicyTargetReference
	GetSpec() PolicySpec

	Merge(Policy, MergeStrategy) Policy
}

// PolicySpec contains a list of policy rules.
// It can be merged with another PolicySpec based on a given MergeStrategy.
type PolicySpec interface {
	DeepCopy() PolicySpec
	SetRules([]Rule)
	GetRules() []Rule

	Merge(PolicySpec, MergeStrategy) PolicySpec
}

// Rule represents a policy rule, containing an ID that uniquely identifies the rule within the policy and a spec.
type Rule interface {
	GetId() RuleId
}

type RuleId string

// MergeStrategy is a function that merges two PolicySpecs into a new PolicySpec.
type MergeStrategy func(PolicySpec, PolicySpec) PolicySpec

// PolicyTargetReference is a generic interface for all kinds of Gateway API policy target references.
// It implements the Object interface for the referent.
type PolicyTargetReference interface {
	Object
}

type NamespacedPolicyTargetReference struct {
	gwapiv1alpha2.NamespacedPolicyTargetReference
	PolicyNamespace string
}

var _ PolicyTargetReference = NamespacedPolicyTargetReference{}

func (t NamespacedPolicyTargetReference) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group: string(t.Group),
		Kind:  string(t.Kind),
	}
}

func (t NamespacedPolicyTargetReference) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	t.Group = gwapiv1alpha2.Group(gvk.Group)
	t.Kind = gwapiv1alpha2.Kind(gvk.Kind)
}

func (t NamespacedPolicyTargetReference) GetURL() string {
	return UrlFromObject(t)
}

func (t NamespacedPolicyTargetReference) GetNamespace() string {
	return string(ptr.Deref(t.Namespace, gwapiv1alpha2.Namespace(t.PolicyNamespace)))
}

func (t NamespacedPolicyTargetReference) GetName() string {
	return string(t.NamespacedPolicyTargetReference.Name)
}

type LocalPolicyTargetReference struct {
	gwapiv1alpha2.LocalPolicyTargetReference
	PolicyNamespace string
}

var _ PolicyTargetReference = LocalPolicyTargetReference{}

func (t LocalPolicyTargetReference) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group: string(t.Group),
		Kind:  string(t.Kind),
	}
}

func (t LocalPolicyTargetReference) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	t.Group = gwapiv1alpha2.Group(gvk.Group)
	t.Kind = gwapiv1alpha2.Kind(gvk.Kind)
}

func (t LocalPolicyTargetReference) GetURL() string {
	return UrlFromObject(t)
}

func (t LocalPolicyTargetReference) GetNamespace() string {
	return t.PolicyNamespace
}

func (t LocalPolicyTargetReference) GetName() string {
	return string(t.LocalPolicyTargetReference.Name)
}

type LocalPolicyTargetReferenceWithSectionName struct {
	gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName
	PolicyNamespace string
}

var _ PolicyTargetReference = LocalPolicyTargetReferenceWithSectionName{}

func (t LocalPolicyTargetReferenceWithSectionName) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group: string(t.Group),
		Kind:  string(t.Kind),
	}
}

func (t LocalPolicyTargetReferenceWithSectionName) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	t.Group = gwapiv1alpha2.Group(gvk.Group)
	t.Kind = gwapiv1alpha2.Kind(gvk.Kind)
}

func (t LocalPolicyTargetReferenceWithSectionName) GetURL() string {
	return UrlFromObject(t)
}

func (t LocalPolicyTargetReferenceWithSectionName) GetNamespace() string {
	return t.PolicyNamespace
}

func (t LocalPolicyTargetReferenceWithSectionName) GetName() string {
	sectionName := ptr.Deref(t.SectionName, gwapiv1alpha2.SectionName(""))
	return namespacedName(string(t.LocalPolicyTargetReference.Name), string(sectionName))
}
