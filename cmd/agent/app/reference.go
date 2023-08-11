package app

import "sync/atomic"

type Reference[T any] struct {
	Object      *T
	count       atomic.Int32
	onCountZero func()
}

func NewReference[T any](object *T, onCountZero func()) *Reference[T] {
	reference := &Reference[T]{
		Object:      object,
		count:       atomic.Int32{},
		onCountZero: onCountZero,
	}

	reference.count.Store(1)
	return reference
}

func (reference *Reference[T]) Acquire() bool {
	return reference.count.Add(1) > 1
}

func (reference *Reference[T]) Release() {
	if reference.count.Add(-1) == 0 {
		reference.onCountZero()
		reference.Object = nil
	}
}
