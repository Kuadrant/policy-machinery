package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kuadrant/policy-machinery/machinery"
)

type Object interface {
	runtime.Object
	metav1.Object
}

// RuntimeObject is a cluster runtime object that implements machinery.Object interface
type RuntimeObject struct {
	Object
}

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
