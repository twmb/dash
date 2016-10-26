// This transliterates Dmitry Vyukov's blocking mpmc queue, which is licensed
// with BSD-3 clause.

// Package spscdvq provides a concurrent single-producer single-consumer fast
// queue based off Dmitry Vyukov's mpmc blocking queue.
//
// This queue is fast, beating throughput of a go channels with high cores.
//
// Queue's are forced to a multiplier-of-two size before returning. If
// enqueueing or dequeueing fails, enqueuers or dequeuers need to backoff
// before attempting enqueueing or dequeueing again. Failing to do so may lead
// to live locks if enqueueing or dequeueing is not be preempted by the go
// scheduler.
package spscdvq

import (
	"unsafe"

	"github.com/twmb/dash/primitive"
)

// See mpmc's mpmcdvq for full comments on the structs and consts.

const cellSz = unsafe.Sizeof(cell{})

type cell struct {
	seq  uintptr
	ptr  unsafe.Pointer
	_pad [primitive.FalseShare - primitive.UpSz]byte
}

// Queue represents a single-producer, single-consumer, fast queue.
type Queue struct {
	_pad0  [primitive.FalseShare - primitive.UpSz]byte
	mask   uintptr
	cells  []cell
	_pad1  [primitive.FalseShare - primitive.UpSz]byte
	enqPos uintptr
	_pad2  [primitive.FalseShare - primitive.UpSz]byte
	deqPos uintptr
	_pad3  [primitive.FalseShare - primitive.UpSz]byte
}

// New returns a new Queue, with size rounded up to the next power of 2.
func New(size uint) *Queue {
	size2 := primitive.Next2(uintptr(size))
	cells := make([]cell, size2+1)
	for i := uintptr(0); i < size2+1; i++ {
		cells[i].seq = i - 1
	}

	q := &Queue{
		mask:  size2 - 1,
		cells: cells[1:],
	}
	return q
}
