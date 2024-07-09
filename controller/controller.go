package controller

import (
	"log"
	"sync"
	"time"

	"github.com/kuadrant/policy-machinery/machinery"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

type ControllerOptions struct {
	client      *dynamic.DynamicClient
	informers   map[string]InformerBuilder
	callback    CallbackFunc
	policyKinds []schema.GroupKind
}

type ControllerOptionFunc func(*ControllerOptions)
type CallbackFunc func(EventType, RuntimeObject, RuntimeObject, *machinery.Topology)

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
		o.policyKinds = policyKinds
	}
}

func NewController(f ...ControllerOptionFunc) *Controller {
	opts := &ControllerOptions{
		informers: map[string]InformerBuilder{},
		callback:  func(EventType, RuntimeObject, RuntimeObject, *machinery.Topology) {},
	}

	for _, fn := range f {
		fn(opts)
	}

	controller := &Controller{
		client:    opts.client,
		cache:     newCacheStore(),
		topology:  NewGatewayAPITopology(opts.policyKinds...),
		informers: map[string]cache.SharedInformer{},
		callback:  opts.callback,
	}

	for name, builder := range opts.informers {
		controller.informers[name] = builder(controller)
	}

	return controller
}

type Controller struct {
	mu        sync.Mutex
	client    *dynamic.DynamicClient
	cache     *cacheStore
	topology  *GatewayAPITopology
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

func (c *Controller) add(obj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Add(obj)
	c.topology.Refresh(c.cache.List())
	c.propagate(CreateEvent, nil, obj)
}

func (c *Controller) update(oldObj, newObj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if oldObj.GetGeneration() == newObj.GetGeneration() {
		return
	}

	c.cache.Add(newObj)
	c.topology.Refresh(c.cache.List())
	c.propagate(UpdateEvent, oldObj, newObj)
}

func (c *Controller) delete(obj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Delete(obj)
	c.topology.Refresh(c.cache.List())
	c.propagate(DeleteEvent, obj, nil)
}

func (c *Controller) propagate(eventType EventType, oldObj, newObj RuntimeObject) {
	c.callback(eventType, oldObj, newObj, c.topology.Get())
}
