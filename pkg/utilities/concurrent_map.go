/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package utilities

import "sync"

type ConcurrentMap[K comparable, V any] struct {
	sync.Mutex

	Map map[K]V
}

func NewConcurrentMap[K comparable, V any]() *ConcurrentMap[K, V] {
	return &ConcurrentMap[K, V]{
		Map: map[K]V{},
	}
}

func (cmap *ConcurrentMap[K, V]) Get(key K) (V, bool) {
	cmap.Lock()
	defer cmap.Unlock()

	value, found := cmap.Map[key]
	return value, found
}

func (cmap *ConcurrentMap[K, V]) Set(key K, value V) {
	cmap.Lock()
	defer cmap.Unlock()

	cmap.Map[key] = value
}

func (cmap *ConcurrentMap[K, V]) Delete(key K) {
	cmap.Lock()
	defer cmap.Unlock()

	delete(cmap.Map, key)
}

func (cmap *ConcurrentMap[K, V]) Foreach(callback func(key K, value V) bool) {
	cmap.Lock()
	defer cmap.Unlock()

	for key, value := range cmap.Map {
		if !callback(key, value) {
			break
		}
	}
}

func (cmap *ConcurrentMap[K, V]) Len() int {
	cmap.Lock()
	defer cmap.Unlock()

	return len(cmap.Map)
}

func (cmap *ConcurrentMap[K, V]) Empty() bool {
	cmap.Lock()
	defer cmap.Unlock()

	return len(cmap.Map) == 0
}

func (cmap *ConcurrentMap[K, V]) Clear() {
	cmap.Lock()
	defer cmap.Unlock()

	cmap.Map = map[K]V{}
}
