package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
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

type RunnableBuilderOptions[T Object] struct {
	LabelSelector string
	FieldSelector string
	Predicates    []ctrlruntimepredicate.TypedPredicate[T]
	Builder       func(obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder
}

type RunnableBuilderOption[T Object] func(*RunnableBuilderOptions[T])

func FilterResourcesByLabel[T Object](selector string) RunnableBuilderOption[T] {
	return func(o *RunnableBuilderOptions[T]) {
		o.LabelSelector = selector
	}
}

func FilterResourcesByField[T Object](selector string) RunnableBuilderOption[T] {
	return func(o *RunnableBuilderOptions[T]) {
		o.FieldSelector = selector
	}
}

func WithPredicates[T Object](predicates ...ctrlruntimepredicate.TypedPredicate[T]) RunnableBuilderOption[T] {
	return func(o *RunnableBuilderOptions[T]) {
		o.Predicates = append(o.Predicates, predicates...)
	}
}

func Builder[T Object](builder func(obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder) RunnableBuilderOption[T] {
	return func(o *RunnableBuilderOptions[T]) {
		o.Builder = builder
	}
}

func Watch[T Object](obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder {
	o := &RunnableBuilderOptions[T]{
		Builder: StateReconciler[T],
	}
	for _, f := range options {
		f(o)
	}
	return o.Builder(obj, resource, namespace, options...)
}

func IncrementalInformer[T Object](obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder {
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

func StateReconciler[T Object](obj T, resource schema.GroupVersionResource, namespace string, options ...RunnableBuilderOption[T]) RunnableBuilder {
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
			listFunc: func() []Object {
				listOptions := metav1.ListOptions{}
				if o.LabelSelector != "" {
					listOptions.LabelSelector = o.LabelSelector
				}
				if o.FieldSelector != "" {
					listOptions.FieldSelector = o.FieldSelector
				}
				objs, err := controller.client.Resource(resource).Namespace(namespace).List(context.Background(), listOptions)
				if err != nil {
					controller.logger.Error(err, "failed to list resources", "kind", kind)
					return nil
				}
				return lo.Map(objs.Items, func(o unstructured.Unstructured, _ int) Object {
					obj, err := Restructure[T](&o)
					if err != nil {
						controller.logger.Error(err, "failed to restructure object", "kind", kind)
						return nil
					}
					runtimeObj, _ := obj.(Object)
					return runtimeObj
				})
			},
			watchFunc: func(manager ctrlruntime.Manager) ctrlruntimesrc.Source {
				var predicates []ctrlruntimepredicate.TypedPredicate[T]
				if o.LabelSelector != "" {
					predicates = append(predicates, ctrlruntimepredicate.NewTypedPredicateFuncs(func(obj T) bool {
						return ToLabelSelector(o.LabelSelector).Matches(labels.Set(obj.GetLabels()))
					}))
				}
				if o.FieldSelector != "" {
					predicates = append(predicates, ctrlruntimepredicate.NewTypedPredicateFuncs(func(obj T) bool {
						selector := ToFieldSelector(o.FieldSelector)
						return selector.Matches(fields.Set(FieldsFromObject(obj, lo.Map(selector.Requirements(), func(r fields.Requirement, _ int) string {
							return r.Field
						}))))
					}))
				}

				// Add custom predicates passed via options
				if len(o.Predicates) > 0 {
					predicates = append(predicates, o.Predicates...)
				}

				return ctrlruntimesrc.Kind(manager.GetCache(), obj, ctrlruntimehandler.TypedEnqueueRequestsFromMapFunc(TypedEnqueueRequestsMapFunc[T]), predicates...)
			},
		}
	}
}

func TypedEnqueueRequestsMapFunc[T Object](_ context.Context, _ T) []ctrlruntimereconcile.Request {
	return []ctrlruntimereconcile.Request{{NamespacedName: types.NamespacedName{}}}
}

type stateReconciler struct {
	controller *Controller
	listFunc   ListFunc
	watchFunc  WatchFunc
	synced     bool
	sync.RWMutex
}

func (r *stateReconciler) Run(_ <-chan struct{}) {
	r.Lock()
	defer r.Unlock()
	r.controller.listAndWatch(r.listFunc, r.watchFunc)
	r.synced = true
}

func (r *stateReconciler) HasSynced() bool {
	r.RLock()
	defer r.RUnlock()

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
		return labels.Nothing()
	} else {
		return selector
	}
}

func ToFieldSelector(s string) fields.Selector {
	if selector, err := fields.ParseSelector(s); err != nil {
		return fields.Nothing()
	} else {
		return selector
	}
}

func FieldsFromObject[T any](obj T, fields []string) map[string]string {
	m := make(map[string]string, len(fields))
	for _, path := range fields {
		parts := strings.SplitN(path, ".", 2)
		field := parts[0]
		rest := strings.Join(parts[1:], ".")
		o, err := Destruct(obj)
		if err != nil {
			continue
		}
		var value string
		switch reflect.TypeOf(o.Object[field]).Kind() {
		case reflect.Struct, reflect.Map:
			if len(rest) > 0 {
				value = FieldsFromObject(o.Object[field], []string{rest})[rest]
			}
		default:
			value = fmt.Sprintf("%v", o.Object[field])
		}
		m[path] = value
	}
	return m
}
