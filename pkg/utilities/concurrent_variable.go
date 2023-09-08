/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package utilities

import "sync"

type ConcurrentVariable[V any] struct {
	sync.Mutex

	Value V
}

func NewConcurrentVariable[V any]() *ConcurrentVariable[V] {
	return &ConcurrentVariable[V]{}
}

func NewConcurrentVariableD[V any](value V) *ConcurrentVariable[V] {
	return &ConcurrentVariable[V]{
		Value: value,
	}
}

func (cvar *ConcurrentVariable[V]) Get() V {
	cvar.Lock()
	defer cvar.Unlock()

	return cvar.Value
}

func (cvar *ConcurrentVariable[V]) Set(value V) {
	cvar.Lock()
	defer cvar.Unlock()

	cvar.Value = value
}

func WithReturn[V any, R any](variable *ConcurrentVariable[V], callback func(value V) R) R {
	variable.Lock()
	defer variable.Unlock()

	return callback(variable.Value)
}

func With[V any](variable *ConcurrentVariable[V], callback func(value V)) {
	variable.Lock()
	defer variable.Unlock()

	callback(variable.Value)
}
