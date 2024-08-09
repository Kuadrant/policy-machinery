package controller

import (
	"reflect"
	"sync"

	"github.com/telepresenceio/watchable"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type RuntimeObject interface {
	runtime.Object
	metav1.Object
}

type RuntimeObjects map[string]RuntimeObject

func (o RuntimeObjects) DeepCopy() RuntimeObjects {
	co := make(RuntimeObjects, len(o))
	for k, v := range o {
		co[k] = v.DeepCopyObject().(RuntimeObject)
	}
	return co
}

func (o RuntimeObjects) Equal(other RuntimeObjects) bool {
	if len(o) != len(other) {
		return false
	}
	for k, v := range o {
		if ov, ok := other[k]; !ok || !reflect.DeepEqual(v, ov) {
			return false
		}
	}
	return true
}

type Cache interface {
	List() Store
	Add(obj RuntimeObject)
	Delete(obj RuntimeObject)
	Replace(Store)
}

type Store map[schema.GroupKind]RuntimeObjects

func newCacheStore() Cache {
	return &watchableCacheStore{}
}

type cacheStore struct {
	mu    sync.RWMutex
	store Store
}

func (c *cacheStore) List() Store {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cm := make(Store, len(c.store))
	for gk, objs := range c.store {
		if _, ok := cm[gk]; !ok {
			cm[gk] = RuntimeObjects{}
		}
		for url, obj := range objs {
			cm[gk][url] = obj
		}
	}
	return cm
}

func (c *cacheStore) Add(obj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
	if _, ok := c.store[gk]; !ok {
		c.store[gk] = RuntimeObjects{}
	}
	c.store[gk][string(obj.GetUID())] = obj
}

func (c *cacheStore) Delete(obj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
	delete(c.store[gk], string(obj.GetUID()))
}

func (c *cacheStore) Replace(store Store) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store = store
}

type watchableCacheStore struct {
	watchable.Map[schema.GroupKind, RuntimeObjects]
}

func (c *watchableCacheStore) List() Store {
	return c.LoadAll()
}

func (c *watchableCacheStore) Add(obj RuntimeObject) {
	gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
	increment := RuntimeObjects{
		string(obj.GetUID()): obj,
	}
	value, loaded := c.LoadOrStore(gk, increment)
	if !loaded {
		return
	}
	value[string(obj.GetUID())] = obj
	c.Store(gk, value)
}

func (c *watchableCacheStore) Delete(obj RuntimeObject) {
	gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
	value, ok := c.Load(gk)
	if !ok {
		return
	}
	delete(value, string(obj.GetUID()))
	c.Store(gk, value)
}

func (c *watchableCacheStore) Replace(store Store) {
	for gk := range c.LoadAll() {
		if objs := store[gk]; len(objs) == 0 {
			c.Map.Delete(gk)
		}
	}
	for gk, objs := range store {
		c.Store(gk, objs)
	}
}
