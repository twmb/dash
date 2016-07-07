// This transliterates Dmitry Vyukov's blocking mpmc queue, which is licensed
// with BSD-3 clause.

// Package spmcdvq provides a concurrent single-producer multi-consumer fast
// queue based off Dmitry Vyukov's mpmc blocking queue.
//
// This queue is fast, beating throughput of a go channels with high cores.
// This queue is not strictly linearizable. If enqueue(a) finishes
// before enqueue(b), dequeue(b) may happen before dequeue(a).
//
// Queue's are forced to a multiplier-of-two size before returning. If
// enqueueing or dequeueing fails, enqueuers or dequeuers need to backoff
// before attempting enqueueing or dequeueing again. Failing to do so may lead
// to live locks if enqueueing or dequeueing is not be preempted by the go
// scheduler.
package spmcdvq

import (
	"reflect"
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

// Queue represents a single-producer, multi-consumer, fast queue.
type Queue struct {
	_pad0  [primitive.FalseShare - primitive.UpSz]byte
	mask   uintptr
	bufPtr unsafe.Pointer
	_pad1  [primitive.FalseShare - primitive.UpSz]byte
	enqPos uintptr
	_pad2  [primitive.FalseShare - primitive.UpSz]byte
	deqPos uintptr
	_pad3  [primitive.FalseShare - primitive.UpSz]byte
}

// New returns a new Queue, with size rounded up to the next power of 2.
func New(size uint) *Queue {
	size2 := primitive.Next2(uintptr(size))
	buf := make([]cell, size2+1)
	for i := uintptr(0); i < size2+1; i++ {
		buf[i].seq = i - 1
	}

	q := &Queue{
		mask:   size2 - 1,
		bufPtr: unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&buf)).Data + cellSz),
	}
	return q
}
