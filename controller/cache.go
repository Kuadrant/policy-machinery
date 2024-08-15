package controller

import (
	"reflect"
	"sync"

	"github.com/samber/lo"
	"github.com/telepresenceio/watchable"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Store map[string]Object

func (s Store) Filter(predicates ...func(Object) bool) []Object {
	var objects []Object
	for _, object := range s {
		if lo.EveryBy(predicates, func(p func(Object) bool) bool { return p(object) }) {
			objects = append(objects, object)
		}
	}
	return objects
}

func (s Store) FilterByGroupKind(gk schema.GroupKind) []Object {
	return s.Filter(func(o Object) bool {
		return o.GetObjectKind().GroupVersionKind().GroupKind() == gk
	})
}

type Cache interface {
	List() Store
	Add(obj Object)
	Delete(obj Object)
	Replace(Store)
}

type cacheStore struct {
	sync.RWMutex
	store Store
}

func (c *cacheStore) List() Store {
	c.RLock()
	defer c.RUnlock()

	ret := make(Store, len(c.store))
	for k, v := range c.store {
		ret[k] = v.DeepCopyObject().(Object)
	}
	return ret
}

func (c *cacheStore) Add(obj Object) {
	c.Lock()
	defer c.Unlock()

	c.store[string(obj.GetUID())] = obj
}

func (c *cacheStore) Delete(obj Object) {
	c.Lock()
	defer c.Unlock()

	delete(c.store, string(obj.GetUID()))
}

func (c *cacheStore) Replace(store Store) {
	c.Lock()
	defer c.Unlock()

	c.store = make(Store, len(store))
	for k, v := range store {
		c.store[k] = v.DeepCopyObject().(Object)
	}
}

type watchableCacheStore struct {
	watchable.Map[string, watchableCacheEntry]
}

func (c *watchableCacheStore) List() Store {
	entries := c.LoadAll()
	store := make(Store, len(entries))
	for uid, obj := range entries {
		store[uid] = obj.Object
	}
	return store
}

func (c *watchableCacheStore) Add(obj Object) {
	c.Store(string(obj.GetUID()), watchableCacheEntry{obj})
}

func (c *watchableCacheStore) Delete(obj Object) {
	c.Map.Delete(string(obj.GetUID()))
}

func (c *watchableCacheStore) Replace(store Store) {
	for uid, obj := range store {
		c.Store(uid, watchableCacheEntry{obj})
	}
	for uid := range c.LoadAll() {
		if _, ok := store[uid]; !ok {
			c.Map.Delete(uid)
		}
	}
}

type watchableCacheEntry struct {
	Object
}

func (e watchableCacheEntry) DeepCopy() watchableCacheEntry {
	return watchableCacheEntry{e.DeepCopyObject().(Object)}
}

func (e watchableCacheEntry) Equal(other watchableCacheEntry) bool {
	return reflect.DeepEqual(e, other)
}
