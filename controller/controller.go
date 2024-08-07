package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"github.com/telepresenceio/watchable"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	ctrlruntime "sigs.k8s.io/controller-runtime"
	ctrlruntimectrl "sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlruntimereconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
	ctrlruntimesrc "sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kuadrant/policy-machinery/machinery"
)

type ResourceEvent struct {
	Kind      schema.GroupKind
	EventType EventType
	OldObject RuntimeObject
	NewObject RuntimeObject
}

type EventType int

const (
	CreateEvent EventType = iota
	UpdateEvent
	DeleteEvent
)

func (t *EventType) String() string {
	return [...]string{"create", "update", "delete"}[*t]
}

type RuntimeLinkFunc func(objs Store) machinery.LinkFunc

type ControllerOptions struct {
	name        string
	logger      logr.Logger
	client      *dynamic.DynamicClient
	manager     ctrlruntime.Manager
	runnables   map[string]RunnableBuilder
	reconcile   ReconcileFunc
	policyKinds []schema.GroupKind
	objectKinds []schema.GroupKind
	objectLinks []RuntimeLinkFunc
}

type ControllerOption func(*ControllerOptions)

type ReconcileFunc func(context.Context, []ResourceEvent, *machinery.Topology)

func WithName(name string) ControllerOption {
	return func(o *ControllerOptions) {
		o.name = name
	}
}

func WithClient(client *dynamic.DynamicClient) ControllerOption {
	return func(o *ControllerOptions) {
		o.client = client
	}
}

func WithLogger(logger logr.Logger) ControllerOption {
	return func(o *ControllerOptions) {
		o.logger = logger
	}
}

func WithRunnable(name string, builder RunnableBuilder) ControllerOption {
	return func(o *ControllerOptions) {
		o.runnables[name] = builder
	}
}

func WithReconcile(reconcile ReconcileFunc) ControllerOption {
	return func(o *ControllerOptions) {
		o.reconcile = reconcile
	}
}

func WithPolicyKinds(policyKinds ...schema.GroupKind) ControllerOption {
	return func(o *ControllerOptions) {
		o.policyKinds = append(o.policyKinds, policyKinds...)
	}
}

func ManagedBy(manager ctrlruntime.Manager) ControllerOption {
	return func(o *ControllerOptions) {
		o.manager = manager
	}
}

func WithObjectKinds(objectKinds ...schema.GroupKind) ControllerOption {
	return func(o *ControllerOptions) {
		o.objectKinds = append(o.objectKinds, objectKinds...)
	}
}

func WithObjectLinks(objectLinks ...RuntimeLinkFunc) ControllerOption {
	return func(o *ControllerOptions) {
		o.objectLinks = append(o.objectLinks, objectLinks...)
	}
}

func NewController(f ...ControllerOption) *Controller {
	opts := &ControllerOptions{
		name:      "controller",
		logger:    logr.Discard(),
		runnables: map[string]RunnableBuilder{},
		reconcile: func(context.Context, []ResourceEvent, *machinery.Topology) {
		},
	}
	for _, fn := range f {
		fn(opts)
	}

	controller := &Controller{
		name:      opts.name,
		logger:    opts.logger,
		client:    opts.client,
		manager:   opts.manager,
		cache:     newCacheStore(),
		topology:  newGatewayAPITopologyBuilder(opts.policyKinds, opts.objectKinds, opts.objectLinks),
		runnables: map[string]Runnable{},
		reconcile: opts.reconcile,
	}

	for name, builder := range opts.runnables {
		controller.runnables[name] = builder(controller)
	}

	return controller
}

type ListFunc func() (schema.GroupKind, RuntimeObjects)
type WatchFunc func(ctrlruntime.Manager) ctrlruntimesrc.Source

type Controller struct {
	sync.Mutex
	name       string
	logger     logr.Logger
	client     *dynamic.DynamicClient
	manager    ctrlruntime.Manager
	cache      Cache
	topology   *gatewayAPITopologyBuilder
	runnables  map[string]Runnable
	listFuncs  []func() (schema.GroupKind, RuntimeObjects)
	watchFuncs []func(ctrlruntime.Manager) ctrlruntimesrc.Source
	reconcile  ReconcileFunc
}

// Start starts the runnables and blocks until a stop signal is received
func (c *Controller) Start() error {
	stopCh := make(chan struct{}, len(c.runnables))

	// subscribe to cache
	c.subscribe()

	// start runnables
	for name := range c.runnables {
		defer close(stopCh)
		c.logger.Info("starting runnable", "name", name)
		go c.runnables[name].Run(stopCh)
	}

	// wait for cache sync
	for name := range c.runnables {
		if !cache.WaitForCacheSync(stopCh, c.runnables[name].HasSynced) {
			return fmt.Errorf("error waiting for %s cache sync", name)
		}
	}

	// start controller manager
	if c.manager != nil {
		ctrl, err := ctrlruntimectrl.New(c.name, c.manager, ctrlruntimectrl.Options{Reconciler: c})
		if err != nil {
			return fmt.Errorf("Error creating controller: %v", err)
		}
		for _, f := range c.watchFuncs {
			if err := ctrl.Watch(f(c.manager)); err != nil {
				return fmt.Errorf("Error watching resource: %v", err)
			}
		}
		c.logger.V(1).Info("starting controller manager")
		c.manager.Start(ctrlruntime.SetupSignalHandler())
		c.logger.V(1).Info("finishing controller manager")
		return nil
	}

	// keep the thread alive
	c.logger.Info("waiting until stop signal is received")
	wait.Until(func() {}, time.Second, stopCh)
	c.logger.Info("stop signal received. finishing controller...")

	return nil
}

func (c *Controller) Reconcile(ctx context.Context, _ ctrlruntimereconcile.Request) (ctrlruntimereconcile.Result, error) {
	c.Lock()
	defer c.Unlock()

	c.logger.Info("reconciling state of the world started")
	defer c.logger.Info("reconciling state of the world finished")

	store := Store{}
	for _, f := range c.listFuncs {
		gk, objects := f()
		store[gk] = objects
	}
	c.cache.Replace(store)

	return ctrlruntimereconcile.Result{}, nil
}

func (c *Controller) listAndWatch(listFunc ListFunc, watchFunc WatchFunc) {
	c.Lock()
	defer c.Unlock()

	c.listFuncs = append(c.listFuncs, listFunc)
	c.watchFuncs = append(c.watchFuncs, watchFunc)
}

func (c *Controller) add(obj RuntimeObject) {
	c.Lock()
	defer c.Unlock()

	c.cache.Add(obj)
	c.propagate([]ResourceEvent{{obj.GetObjectKind().GroupVersionKind().GroupKind(), CreateEvent, nil, obj}})
}

func (c *Controller) update(oldObj, newObj RuntimeObject) {
	c.Lock()
	defer c.Unlock()

	if oldObj.GetGeneration() == newObj.GetGeneration() {
		return
	}

	c.cache.Add(newObj)
	c.propagate([]ResourceEvent{{newObj.GetObjectKind().GroupVersionKind().GroupKind(), UpdateEvent, oldObj, newObj}})
}

func (c *Controller) delete(obj RuntimeObject) {
	c.Lock()
	defer c.Unlock()

	c.cache.Delete(obj)
	c.propagate([]ResourceEvent{{obj.GetObjectKind().GroupVersionKind().GroupKind(), DeleteEvent, obj, nil}})
}

func (c *Controller) propagate(resourceEvents []ResourceEvent) {
	topology := c.topology.Build(c.cache.List())
	c.reconcile(LoggerIntoContext(context.TODO(), c.logger), resourceEvents, topology)
}

func (c *Controller) subscribe() {
	cache, ok := c.cache.(*watchableCacheStore) // TODO(guicassolato): decide if we should extend the Cache interface or remove it altogether
	if !ok {
		return
	}
	subscription := cache.Subscribe(context.TODO())
	go func() {
		for snapshot := range subscription {
			c.Lock()

			c.propagate(lo.FlatMap(snapshot.Updates, func(update watchable.Update[schema.GroupKind, RuntimeObjects], _ int) []ResourceEvent {
				var events []ResourceEvent

				eventType := UpdateEvent // what about CreateEvent?
				if update.Delete {
					eventType = DeleteEvent
				}

				for _, obj := range update.Value {
					event := ResourceEvent{
						Kind:      update.Key,
						EventType: eventType,
					}
					switch eventType {
					case CreateEvent:
						event.NewObject = obj
					case UpdateEvent:
						event.OldObject = nil // what about previous state?
						event.NewObject = obj
					case DeleteEvent:
						event.OldObject = obj
					}
					events = append(events, event)
				}
				return events
			}))

			c.Unlock()
		}
	}()
}
