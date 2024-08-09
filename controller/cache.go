package controller

import (
	"reflect"
	"sync"

	"github.com/samber/lo"
	"github.com/telepresenceio/watchable"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type RuntimeObject interface {
	runtime.Object
	metav1.Object
}

// RuntimeObjectAs casts a RuntimeObject generically into any kind
func RuntimeObjectAs[T any](obj RuntimeObject, _ int) T {
	o, _ := obj.(T)
	return o
}

type Store map[string]RuntimeObject

func (s Store) List(predicates ...func(RuntimeObject) bool) []RuntimeObject {
	var objects []RuntimeObject
	for _, object := range s {
		if lo.EveryBy(predicates, func(p func(RuntimeObject) bool) bool { return p(object) }) {
			objects = append(objects, object)
		}
	}
	return objects
}

func (s Store) ListByGroupKind(gk schema.GroupKind) []RuntimeObject {
	return s.List(func(o RuntimeObject) bool {
		return o.GetObjectKind().GroupVersionKind().GroupKind() == gk
	})
}

type Cache interface {
	List() Store
	Add(obj RuntimeObject)
	Delete(obj RuntimeObject)
	Replace(Store)
}

func newCacheStore() Cache {
	return &watchableCacheStore{}
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
		ret[k] = v.DeepCopyObject().(RuntimeObject)
	}
	return ret
}

func (c *cacheStore) Add(obj RuntimeObject) {
	c.Lock()
	defer c.Unlock()

	c.store[string(obj.GetUID())] = obj
}

func (c *cacheStore) Delete(obj RuntimeObject) {
	c.Lock()
	defer c.Unlock()

	delete(c.store, string(obj.GetUID()))
}

func (c *cacheStore) Replace(store Store) {
	c.Lock()
	defer c.Unlock()

	c.store = make(Store, len(store))
	for k, v := range store {
		c.store[k] = v.DeepCopyObject().(RuntimeObject)
	}
}

type watchableCacheStore struct {
	watchable.Map[string, watchableCacheEntry]
}

func (c *watchableCacheStore) List() Store {
	entries := c.LoadAll()
	store := make(Store, len(entries))
	for uid, obj := range entries {
		store[uid] = obj.RuntimeObject
	}
	return store
}

func (c *watchableCacheStore) Add(obj RuntimeObject) {
	c.Store(string(obj.GetUID()), watchableCacheEntry{obj})
}

func (c *watchableCacheStore) Delete(obj RuntimeObject) {
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
	RuntimeObject
}

func (e watchableCacheEntry) DeepCopy() watchableCacheEntry {
	return watchableCacheEntry{e.DeepCopyObject().(RuntimeObject)}
}

func (e watchableCacheEntry) Equal(other watchableCacheEntry) bool {
	return reflect.DeepEqual(e, other)
}
