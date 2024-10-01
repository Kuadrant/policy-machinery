package controller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/kuadrant/policy-machinery/machinery"
)

type Object interface {
	runtime.Object
	metav1.Object
}

// RuntimeObject is a wrapper around a Kubernetes runtime object, that also implements the machinery.Object interface
// Use it for wrapping runtime objects that do not natively implement machinery.Object, so such object can be added to
// a machinery.Topology
type RuntimeObject struct {
	Object
}

var _ machinery.Object = &RuntimeObject{}

func (o *RuntimeObject) GroupVersionKind() schema.GroupVersionKind {
	return o.Object.GetObjectKind().GroupVersionKind()
}

func (o *RuntimeObject) SetGroupVersionKind(schema.GroupVersionKind) {}

func (o *RuntimeObject) GetNamespace() string {
	return o.Object.GetNamespace()
}

func (o *RuntimeObject) GetName() string {
	return o.Object.GetName()
}

func (o *RuntimeObject) GetLocator() string {
	return machinery.LocatorFromObject(o)
}

// ObjectAs casts an Object generically into any kind
func ObjectAs[T any](obj Object, _ int) T {
	o, _ := obj.(T)
	return o
}

// ObjectsByCreationTimestamp is a slice of RuntimeObject that can be sorted by creation timestamp
// RuntimeObjects with the oldest creation timestamp will appear first; if two objects have the same creation timestamp,
// the object appearing first in alphabetical order by namespace/name will appear first.
type ObjectsByCreationTimestamp []Object

func (a ObjectsByCreationTimestamp) Len() int      { return len(a) }
func (a ObjectsByCreationTimestamp) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ObjectsByCreationTimestamp) Less(i, j int) bool {
	p1Time := ptr.To(a[i].GetCreationTimestamp())
	p2Time := ptr.To(a[j].GetCreationTimestamp())
	if !p1Time.Equal(p2Time) {
		return p1Time.Before(p2Time)
	}
	//  The object appearing first in alphabetical order by "{namespace}/{name}".
	return fmt.Sprintf("%s/%s", a[i].GetNamespace(), a[i].GetName()) < fmt.Sprintf("%s/%s", a[j].GetNamespace(), a[j].GetName())
}
