package machinery

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

const kindNameURLSeparator = ':'

type Object interface {
	schema.ObjectKind

	GetNamespace() string
	GetName() string
	GetURL() string
}

func UrlFromObject(obj Object) string {
	name := strings.TrimPrefix(namespacedName(obj.GetNamespace(), obj.GetName()), string(k8stypes.Separator))
	return fmt.Sprintf("%s%s%s", strings.ToLower(obj.GroupVersionKind().GroupKind().String()), string(kindNameURLSeparator), name)
}

func AsObject[T Object](t T, _ int) Object {
	return t
}

func namespacedName(namespace, name string) string {
	return k8stypes.NamespacedName{Namespace: namespace, Name: name}.String()
}

// Targetable is an interface that represents an object that can be targeted by policies.
type Targetable interface {
	Object

	SetPolicies([]Policy)
	Policies() []Policy
}

func MapTargetableToURLFunc(t Targetable, _ int) string {
	return t.GetURL()
}

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

// MergeStrategy is a function that merges two Policy objects into a new Policy object.
type MergeStrategy func(Policy, Policy) Policy

var DefaultMergeStrategy = NoMergeStrategy

func NoMergeStrategy(_, target Policy) Policy {
	return target
}

var _ MergeStrategy = NoMergeStrategy
