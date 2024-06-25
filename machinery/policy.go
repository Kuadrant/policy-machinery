package machinery

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// Policy targets objects and can be merged with another Policy based on a given MergeStrategy.
type Policy interface {
	Object

	GetTargetRefs() []PolicyTargetReference
	GetMergeStrategy() MergeStrategy

	Merge(Policy) Policy
}

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
	if t.SectionName == nil {
		return string(t.LocalPolicyTargetReference.Name)
	}
	return namespacedSectionName(string(t.LocalPolicyTargetReference.Name), *t.SectionName)
}

// MergeStrategy is a function that merges two Policy objects into a new Policy object.
type MergeStrategy func(Policy, Policy) Policy

var DefaultMergeStrategy = NoMergeStrategy

func NoMergeStrategy(_, target Policy) Policy {
	return target
}

var _ MergeStrategy = NoMergeStrategy
