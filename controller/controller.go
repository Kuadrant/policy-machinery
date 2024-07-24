package controller

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/kuadrant/policy-machinery/machinery"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

type ResourceEvent struct {
	Resource  schema.GroupVersionResource
	EventType EventType
	OldObject RuntimeObject
	NewObject RuntimeObject
}

type RuntimeLinkFunc func(objs Store) machinery.LinkFunc

type ControllerOptions struct {
	client      *dynamic.DynamicClient
	informers   map[string]InformerBuilder
	callback    CallbackFunc
	policyKinds []schema.GroupKind
	objectKinds []schema.GroupKind
	objectLinks []RuntimeLinkFunc
}

type ControllerOptionFunc func(*ControllerOptions)
type CallbackFunc func(context.Context, ResourceEvent, *machinery.Topology)

func WithClient(client *dynamic.DynamicClient) ControllerOptionFunc {
	return func(o *ControllerOptions) {
		o.client = client
	}
}

func WithInformer(name string, informer InformerBuilder) ControllerOptionFunc {
	return func(o *ControllerOptions) {
		o.informers[name] = informer
	}
}

func WithCallback(callback CallbackFunc) ControllerOptionFunc {
	return func(o *ControllerOptions) {
		o.callback = callback
	}
}

func WithPolicyKinds(policyKinds ...schema.GroupKind) ControllerOptionFunc {
	return func(o *ControllerOptions) {
		o.policyKinds = append(o.policyKinds, policyKinds...)
	}
}

func WithObjectKinds(objectKinds ...schema.GroupKind) ControllerOptionFunc {
	return func(o *ControllerOptions) {
		o.objectKinds = append(o.objectKinds, objectKinds...)
	}
}

func WithObjectLinks(objectLinks ...RuntimeLinkFunc) ControllerOptionFunc {
	return func(o *ControllerOptions) {
		o.objectLinks = append(o.objectLinks, objectLinks...)
	}
}

func NewController(f ...ControllerOptionFunc) *Controller {
	opts := &ControllerOptions{
		informers: map[string]InformerBuilder{},
		callback: func(context.Context, ResourceEvent, *machinery.Topology) {
		},
	}

	for _, fn := range f {
		fn(opts)
	}

	controller := &Controller{
		client:    opts.client,
		cache:     newCacheStore(),
		topology:  newGatewayAPITopologyBuilder(opts.policyKinds, opts.objectKinds, opts.objectLinks),
		informers: map[string]cache.SharedInformer{},
		callback:  opts.callback,
	}

	for name, builder := range opts.informers {
		controller.informers[name] = builder(controller)
	}

	return controller
}

type Controller struct {
	mu        sync.RWMutex
	client    *dynamic.DynamicClient
	cache     *cacheStore
	topology  *gatewayAPITopologyBuilder
	informers map[string]cache.SharedInformer
	callback  CallbackFunc
}

// Starts starts the informers and blocks until a stop signal is received
func (c *Controller) Start() {
	stopCh := make(chan struct{}, len(c.informers))

	for name := range c.informers {
		defer close(stopCh)
		log.Printf("Starting %s informer", name)
		go c.informers[name].Run(stopCh)
	}

	// wait for stop signal
	for name := range c.informers {
		if !cache.WaitForCacheSync(stopCh, c.informers[name].HasSynced) {
			log.Fatalf("Error waiting for %s cache sync", name)
		}
	}

	// keep the thread alive
	wait.Until(func() {}, time.Second, stopCh)
}

func (c *Controller) add(resource schema.GroupVersionResource, obj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Add(obj)
	c.propagate(ResourceEvent{resource, CreateEvent, nil, obj})
}

func (c *Controller) update(resource schema.GroupVersionResource, oldObj, newObj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if oldObj.GetGeneration() == newObj.GetGeneration() {
		return
	}

	c.cache.Add(newObj)
	c.propagate(ResourceEvent{resource, UpdateEvent, oldObj, newObj})
}

func (c *Controller) delete(resource schema.GroupVersionResource, obj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Delete(obj)
	c.propagate(ResourceEvent{resource, DeleteEvent, obj, nil})
}

func (c *Controller) propagate(resourceEvent ResourceEvent) {
	topology := c.topology.Build(c.cache.List())
	c.callback(context.TODO(), resourceEvent, topology)
}
