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

func (s Store) DeepCopy() Store {
	return lo.SliceToMap(lo.Keys(s), func(uid string) (string, Object) {
		return uid, s[uid].DeepCopyObject().(Object)
	})
}

func (s Store) Equal(other Store) bool {
	return len(s) == len(other) && lo.EveryBy(lo.Keys(s), func(uid string) bool {
		otherObj, ok := other[uid]
		return ok && reflect.DeepEqual(s[uid], otherObj)
	})
}

type CacheStore struct {
	sync.RWMutex
	watchable.Map[string, Store]
}

func (c *CacheStore) List(storeId string) Store {
	c.RLock()
	defer c.RUnlock()
	store, _ := c.Load(storeId)
	return store
}

func (c *CacheStore) Add(storeId string, obj Object) {
	c.Lock()
	defer c.Unlock()
	uid := string(obj.GetUID())
	store, _ := c.Load(storeId)
	store[uid] = obj
	c.Store(storeId, store)
}

func (c *CacheStore) Delete(storeId string, obj Object) {
	c.Lock()
	defer c.Unlock()
	store, _ := c.Load(storeId)
	delete(store, string(obj.GetUID()))
	c.Store(storeId, store)
}

func (c *CacheStore) Replace(storeId string, store Store) {
	c.Lock()
	defer c.Unlock()
	c.Store(storeId, store)
}
