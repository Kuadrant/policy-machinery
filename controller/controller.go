package controller

import (
	"context"
	"fmt"
	"reflect"
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

const resourceStoreId = "resources"

type ControllerOptions struct {
	name               string
	logger             logr.Logger
	client             *dynamic.DynamicClient
	manager            ctrlruntime.Manager
	runnables          map[string]RunnableBuilder
	reconcile          ReconcileFunc
	policyKinds        []schema.GroupKind
	objectKinds        []schema.GroupKind
	objectLinks        []LinkFunc
	allowTopologyLoops bool
}

type ControllerOption func(*ControllerOptions)

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

// ReconcileFunc is a function that reconciles a particular state of the world.
// It receives a list of recent events, an immutable copy of the topology as known by the caller after the events,
// an optional error detected before the reconciliation, and a thread-safe map to store transient state across
// chained calls to multiple ReconcileFuncs.
// If a ReconcileFunc returns an error, a chained sequence of ReconcileFuncs must be interrupted.
type ReconcileFunc func(context.Context, []ResourceEvent, *machinery.Topology, error, *sync.Map) error

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

func WithObjectKinds(objectKinds ...schema.GroupKind) ControllerOption {
	return func(o *ControllerOptions) {
		o.objectKinds = append(o.objectKinds, objectKinds...)
	}
}

type LinkFunc func(objs Store) machinery.LinkFunc

func WithObjectLinks(objectLinks ...LinkFunc) ControllerOption {
	return func(o *ControllerOptions) {
		o.objectLinks = append(o.objectLinks, objectLinks...)
	}
}

func ManagedBy(manager ctrlruntime.Manager) ControllerOption {
	return func(o *ControllerOptions) {
		o.manager = manager
	}
}

func AllowLoops() ControllerOption {
	return func(o *ControllerOptions) {
		o.allowTopologyLoops = true
	}
}

func NewController(f ...ControllerOption) *Controller {
	opts := &ControllerOptions{
		name:      "controller",
		logger:    logr.Discard(),
		runnables: map[string]RunnableBuilder{},
		reconcile: func(context.Context, []ResourceEvent, *machinery.Topology, error, *sync.Map) error {
			return nil
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
		cache:     &CacheStore{},
		topology:  newGatewayAPITopologyBuilder(opts.policyKinds, opts.objectKinds, opts.objectLinks, opts.allowTopologyLoops),
		runnables: map[string]Runnable{},
		reconcile: opts.reconcile,
	}

	for name, builder := range opts.runnables {
		controller.runnables[name] = builder(controller)
	}

	return controller
}

type ListFunc func() []Object
type WatchFunc func(ctrlruntime.Manager) ctrlruntimesrc.Source

type Controller struct {
	sync.Mutex
	name       string
	logger     logr.Logger
	client     *dynamic.DynamicClient
	manager    ctrlruntime.Manager
	cache      *CacheStore
	topology   *gatewayAPITopologyBuilder
	runnables  map[string]Runnable
	listFuncs  []ListFunc
	watchFuncs []WatchFunc
	reconcile  ReconcileFunc
}

// Start starts the runnables and blocks until the context is cancelled
func (c *Controller) Start(ctx context.Context) error {
	stopCh := make(chan struct{})

	// subscribe to cache
	c.subscribe(ctx)

	// start runnables
	for name := range c.runnables {
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
		c.manager.Start(ctx)
		c.logger.V(1).Info("finishing controller manager")
		return nil
	}

	// keep the thread alive
	c.logger.Info("waiting until stop signal is received")
	wait.Until(func() {
		select {
		case <-ctx.Done():
			close(stopCh)
		}
	}, time.Second, stopCh)
	c.logger.Info("stop signal received. finishing controller...")

	return nil
}

func (c *Controller) Reconcile(ctx context.Context, _ ctrlruntimereconcile.Request) (ctrlruntimereconcile.Result, error) {
	c.Lock()
	defer c.Unlock()

	c.logger.V(1).Info("reading state of the world")
	defer c.logger.V(1).Info("finished reading state of the world")

	store := Store{}
	for _, f := range c.listFuncs {
		for _, object := range f() {
			store[string(object.GetUID())] = object
		}
	}
	c.cache.Replace(resourceStoreId, store)

	return ctrlruntimereconcile.Result{}, nil
}

func (c *Controller) listAndWatch(listFunc ListFunc, watchFunc WatchFunc) {
	c.Lock()
	defer c.Unlock()

	c.listFuncs = append(c.listFuncs, listFunc)
	c.watchFuncs = append(c.watchFuncs, watchFunc)
}

func (c *Controller) add(obj Object) {
	c.Lock()
	defer c.Unlock()

	c.cache.Add(resourceStoreId, obj)
}

func (c *Controller) update(_, newObj Object) {
	c.Lock()
	defer c.Unlock()

	c.cache.Add(resourceStoreId, newObj)
}

func (c *Controller) delete(obj Object) {
	c.Lock()
	defer c.Unlock()

	c.cache.Delete(resourceStoreId, obj)
}

func (c *Controller) propagate(resourceEvents []ResourceEvent) {
	c.logger.V(1).Info("propagating new state of the world events", "events", len(resourceEvents))
	defer c.logger.V(1).Info("finished propagating new state of the world events")

	topology, err := c.topology.Build(c.cache.List(resourceStoreId))
	if err != nil {
		c.logger.Error(err, "error building topology")
	}
	if err := c.reconcile(LoggerIntoContext(context.TODO(), c.logger), resourceEvents, topology, err, &sync.Map{}); err != nil {
		c.logger.Error(err, "reconciliation error")
	}
}

func (c *Controller) subscribe(ctx context.Context) {
	// init and subscribe resource store
	c.cache.LoadOrStore(resourceStoreId, Store{})
	subscription := c.cache.SubscribeSubset(ctx, func(storeId string, _ Store) bool {
		return storeId == resourceStoreId
	})
	// handle cache events
	objs := make(Store)
	go func() {
		for snapshot := range subscription {
			objs = c.handleCacheEvent(snapshot, objs)
		}
	}()
}

func (c *Controller) handleCacheEvent(snapshot watchable.Snapshot[string, Store], objs Store) Store {
	c.Lock()
	defer c.Unlock()

	if len(snapshot.Updates) == 0 {
		return objs
	}

	c.logger.V(1).Info("handling new state of the world")
	defer c.logger.V(1).Info("finished handling new state of the world")

	newObjs := snapshot.State[resourceStoreId]

	events := lo.FilterMap(lo.Keys(newObjs), func(uid string, _ int) (ResourceEvent, bool) {
		newObj := newObjs[uid]
		event := ResourceEvent{
			Kind:      newObj.GetObjectKind().GroupVersionKind().GroupKind(),
			NewObject: newObj,
		}
		if obj, exists := objs[uid]; !exists {
			event.EventType = CreateEvent
			objs[uid] = newObj
			return event, true
		} else if !reflect.DeepEqual(obj, newObj) {
			event.EventType = UpdateEvent
			event.OldObject = obj
			objs[uid] = newObj
			return event, true
		}
		return event, false
	})

	deleteEvents := lo.FilterMap(lo.Keys(objs), func(uid string, _ int) (ResourceEvent, bool) {
		obj := objs[uid]
		event := ResourceEvent{
			EventType: DeleteEvent,
			Kind:      obj.GetObjectKind().GroupVersionKind().GroupKind(),
			OldObject: obj,
		}
		_, exists := newObjs[uid]
		if !exists {
			delete(objs, uid)
		}
		return event, !exists
	})

	events = append(events, deleteEvents...)

	if len(events) > 0 { // this condition is actually redundant; if the snapshot has updates, there must be events
		c.propagate(events)
	} else {
		c.logger.V(1).Info("state of the world has not changed")
	}

	return objs
}
