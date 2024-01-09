/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package utilities

import (
	"github.com/Xdevlab/Run/pkg/logger"
)

type Node[T any] struct {
	Data T

	N, P *Node[T]
}

type NodeIterator[T any] struct {
	node  *Node[T]
	first bool
}

func (iterator *NodeIterator[T]) Node() *Node[T] {
	return iterator.node
}

func (iterator *NodeIterator[T]) Value() *T {
	if iterator.node != nil {
		return &iterator.node.Data
	}

	return nil
}

func (iterator *NodeIterator[T]) Next() bool {
	if iterator.first {
		iterator.first = false
		return iterator.node != nil
	}

	if iterator.node != nil {
		iterator.node = iterator.node.N
	}

	return iterator.node != nil
}

type LinkedList[T any] struct {
	allocations [][]Node[T]

	count    int
	capacity int

	head, tail         *Node[T]
	freeHead, freeTail *Node[T]
}

func NewLinkedList[T any]() *LinkedList[T] {
	return &LinkedList[T]{}
}

func (list *LinkedList[T]) Iterator() NodeIterator[T] {
	return NodeIterator[T]{list.head, true}
}

func (list *LinkedList[T]) ensureCapacity(newElements int) {
	if (list.count + newElements) > list.capacity {
		newAllocation := make([]Node[T], 64)
		list.allocations = append(list.allocations, newAllocation)

		prev := list.freeTail
		newAllocation[0].P = prev
		if prev != nil {
			prev.N = &newAllocation[0]
		}

		prev = &newAllocation[0]

		for i := 1; i < len(newAllocation); i++ {
			newAllocation[i].P = prev
			prev.N = &newAllocation[i]

			prev = prev.N
		}

		list.freeTail = prev

		if list.freeHead == nil {
			list.freeHead = &newAllocation[0]
		}

		list.capacity += 64
	}
}

func (list *LinkedList[T]) popFreeNode() *Node[T] {
	if list.freeHead == nil {
		logger.Panic("freeHead should not be nil")
	}

	head := list.freeHead

	list.freeHead = head.N
	list.freeHead.P = nil

	head.N = nil
	head.P = nil
	return head
}

func (list *LinkedList[T]) Append(data T) *Node[T] {
	list.ensureCapacity(1)

	node := list.popFreeNode()
	node.Data = data

	prev := list.tail
	node.P = prev

	if prev != nil {
		prev.N = node
	} else {
		list.head = node
	}

	list.tail = node

	list.count += 1

	return node
}

func (list *LinkedList[T]) AppendMany(data []T) {
	list.ensureCapacity(len(data))

	prev := list.tail
	for _, element := range data {
		node := list.popFreeNode()
		node.Data = element

		node.P = prev

		if prev != nil {
			prev.N = node
		} else {
			list.head = node
		}

		prev = node
	}

	list.tail = prev

	list.count += len(data)
}

func (list *LinkedList[T]) Remove(iterator NodeIterator[T]) NodeIterator[T] {
	if iterator.node != nil {
		node := list.RemoveNode(iterator.node)
		iterator = NodeIterator[T]{node, node != nil}
	}

	return iterator
}

func (list *LinkedList[T]) RemoveNode(current *Node[T]) *Node[T] {
	if current == nil {
		logger.Panic()
	}

	prev := current.P
	next := current.N

	if prev != nil {
		prev.N = next
	} else {
		list.head = next
	}

	if next != nil {
		next.P = prev
	} else {
		list.tail = prev
	}

	current.P = list.freeTail

	if list.freeTail != nil {
		list.freeTail.N = current
	} else {
		list.freeHead = current
	}

	list.freeTail = current

	return next
}
