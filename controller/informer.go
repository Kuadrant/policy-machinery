package controller

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type EventType int

const (
	CreateEvent EventType = iota
	UpdateEvent
	DeleteEvent
)

func (t *EventType) String() string {
	return [...]string{"create", "update", "delete"}[*t]
}

type InformerBuilder func(controller *Controller) cache.SharedInformer

func For[T RuntimeObject](resource schema.GroupVersionResource, namespace string) InformerBuilder {
	return func(controller *Controller) cache.SharedInformer {
		informer := cache.NewSharedInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return controller.client.Resource(resource).Namespace(namespace).List(context.Background(), options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return controller.client.Resource(resource).Namespace(namespace).Watch(context.Background(), options)
				},
			},
			&unstructured.Unstructured{},
			time.Minute*10,
		)
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(o any) {
				obj := o.(T)
				controller.add(obj)
			},
			UpdateFunc: func(o, newO any) {
				oldObj := o.(T)
				newObj := newO.(T)
				controller.update(oldObj, newObj)
			},
			DeleteFunc: func(o any) {
				obj := o.(T)
				controller.delete(obj)
			},
		})
		informer.SetTransform(restructure[T])
		return informer
	}
}

func restructure[T any](obj any) (any, error) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}
	o := *new(T)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &o); err != nil {
		return nil, err
	}
	return o, nil
}
