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

type Runnable interface {
	Run(stopCh <-chan struct{})
	HasSynced() bool
}

type RunnableBuilderOptions struct {
	LabelSelector string
	FieldSelector string
}

type RunnableBuilderOptionsFunc func(*RunnableBuilderOptions)

func FilterResourcesByLabel(selector string) RunnableBuilderOptionsFunc {
	return func(o *RunnableBuilderOptions) {
		o.LabelSelector = selector
	}
}

func FilterResourcesByField(selector string) RunnableBuilderOptionsFunc {
	return func(o *RunnableBuilderOptions) {
		o.FieldSelector = selector
	}
}

type RunnableBuilder func(controller *Controller) Runnable

func Watch[T RuntimeObject](resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOptionsFunc) RunnableBuilder {
	o := &RunnableBuilderOptions{}
	for _, f := range options {
		f(o)
	}
	return func(controller *Controller) Runnable {
		informer := cache.NewSharedInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					if o.LabelSelector != "" {
						options.LabelSelector = o.LabelSelector
					}
					if o.FieldSelector != "" {
						options.FieldSelector = o.FieldSelector
					}
					return controller.client.Resource(resource).Namespace(namespace).List(context.Background(), options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					if o.LabelSelector != "" {
						options.LabelSelector = o.LabelSelector
					}
					if o.FieldSelector != "" {
						options.FieldSelector = o.FieldSelector
					}
					return controller.client.Resource(resource).Namespace(namespace).Watch(context.Background(), options)
				},
			},
			&unstructured.Unstructured{},
			time.Minute*10,
		)
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(o any) {
				obj := o.(T)
				controller.add(resource, obj)
			},
			UpdateFunc: func(o, newO any) {
				oldObj := o.(T)
				newObj := newO.(T)
				controller.update(resource, oldObj, newObj)
			},
			DeleteFunc: func(o any) {
				obj := o.(T)
				controller.delete(resource, obj)
			},
		})
		informer.SetTransform(Restructure[T])
		return informer
	}
}

func Restructure[T any](obj any) (any, error) {
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

func Destruct[T any](obj T) (*unstructured.Unstructured, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: u}, nil
}
