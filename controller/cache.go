package controller

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type RuntimeObject interface {
	runtime.Object
	metav1.Object
}

type Cache interface {
	List() Store
	Add(obj RuntimeObject)
	Delete(obj RuntimeObject)
}

type Store map[schema.GroupKind]map[string]RuntimeObject

type cacheStore struct {
	mu    sync.RWMutex
	store Store
}

func newCacheStore() Cache {
	return &cacheStore{
		store: make(Store),
	}
}

func (c *cacheStore) List() Store {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cm := make(Store, len(c.store))
	for gk, objs := range c.store {
		if _, ok := cm[gk]; !ok {
			cm[gk] = map[string]RuntimeObject{}
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
		c.store[gk] = map[string]RuntimeObject{}
	}
	c.store[gk][string(obj.GetUID())] = obj
}

func (c *cacheStore) Delete(obj RuntimeObject) {
	c.mu.Lock()
	defer c.mu.Unlock()

	gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
	delete(c.store[gk], string(obj.GetUID()))
}
