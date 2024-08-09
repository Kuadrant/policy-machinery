package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	ctrlruntime "sigs.k8s.io/controller-runtime"
	ctrlruntimehandler "sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlruntimepredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
	ctrlruntimereconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
	ctrlruntimesrc "sigs.k8s.io/controller-runtime/pkg/source"
)

type Runnable interface {
	Run(stopCh <-chan struct{})
	HasSynced() bool
}

type RunnableBuilder func(controller *Controller) Runnable

type RunnableBuilderOptions[T RuntimeObject] struct {
	LabelSelector string
	FieldSelector string
	Builder       func(obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder
}

type RunnableBuilderOption[T RuntimeObject] func(*RunnableBuilderOptions[T])

func FilterResourcesByLabel[T RuntimeObject](selector string) RunnableBuilderOption[T] {
	return func(o *RunnableBuilderOptions[T]) {
		o.LabelSelector = selector
	}
}

func FilterResourcesByField[T RuntimeObject](selector string) RunnableBuilderOption[T] {
	return func(o *RunnableBuilderOptions[T]) {
		o.FieldSelector = selector
	}
}

func Builder[T RuntimeObject](builder func(obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder) RunnableBuilderOption[T] {
	return func(o *RunnableBuilderOptions[T]) {
		o.Builder = builder
	}
}

func Watch[T RuntimeObject](obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder {
	o := &RunnableBuilderOptions[T]{
		Builder: StateReconciler[T],
	}
	for _, f := range options {
		f(o)
	}
	return o.Builder(obj, resource, namespace, options...)
}

func IncrementalInformer[T RuntimeObject](obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder {
	o := &RunnableBuilderOptions[T]{}
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
		informer.SetTransform(Restructure[T])
		return informer
	}
}

func StateReconciler[T RuntimeObject](obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder {
	o := &RunnableBuilderOptions[T]{}
	for _, f := range options {
		f(o)
	}

	// extract the kind of resource from the sample object
	// not using obj.GetObjectKind().GroupVersionKind().Kind because the sample object usually does not have it set
	kind := reflect.TypeOf(obj).String()
	kind = kind[strings.LastIndex(kind, ".")+1:]

	return func(controller *Controller) Runnable {
		return &stateReconciler{
			controller: controller,
			listFunc: func() []RuntimeObject {
				listOptions := metav1.ListOptions{}
				if o.LabelSelector != "" {
					listOptions.LabelSelector = o.LabelSelector
				}
				if o.FieldSelector != "" {
					listOptions.FieldSelector = o.FieldSelector
				}
				objs, _ := controller.client.Resource(resource).Namespace(namespace).List(context.Background(), listOptions)
				return lo.Map(objs.Items, func(o unstructured.Unstructured, _ int) RuntimeObject {
					obj, err := Restructure[T](&o)
					if err != nil {
						// TODO: log error
						return nil
					}
					runtimeObj, _ := obj.(RuntimeObject)
					return runtimeObj
				})
			},
			watchFunc: func(manager ctrlruntime.Manager) ctrlruntimesrc.Source {
				predicates := []ctrlruntimepredicate.TypedPredicate[T]{
					&ctrlruntimepredicate.TypedGenerationChangedPredicate[T]{},
				}
				if o.LabelSelector != "" {
					predicates = append(predicates, ctrlruntimepredicate.NewTypedPredicateFuncs(func(obj T) bool {
						return ToLabelSelector(o.LabelSelector).Matches(labels.Set(obj.GetLabels()))
					}))
				}
				// TODO(guicassolato): field selector predicate
				return ctrlruntimesrc.Kind(manager.GetCache(), obj, ctrlruntimehandler.TypedEnqueueRequestsFromMapFunc(TypedEnqueueRequestsMapFunc[T]), predicates...)
			},
		}
	}
}

func TypedEnqueueRequestsMapFunc[T RuntimeObject](_ context.Context, _ T) []ctrlruntimereconcile.Request {
	return []ctrlruntimereconcile.Request{{NamespacedName: types.NamespacedName{}}}
}

type stateReconciler struct {
	controller *Controller
	listFunc   ListFunc
	watchFunc  WatchFunc
	synced     bool
}

func (r *stateReconciler) Run(_ <-chan struct{}) {
	r.controller.listAndWatch(r.listFunc, r.watchFunc)
	r.synced = true
}

func (r *stateReconciler) HasSynced() bool {
	return r.synced
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

func ToLabelSelector(s string) labels.Selector {
	if selector, err := labels.Parse(s); err != nil {
		return labels.NewSelector()
	} else {
		return selector
	}
}
