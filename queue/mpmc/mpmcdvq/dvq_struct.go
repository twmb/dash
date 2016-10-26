// This transliterates Dmitry Vyukov's blocking mpmc queue, which is licensed
// with BSD-3 clause.

// Package mpmcdvq provides a concurrent multi-producer multi-consumer fast
// queue based off Dmitry Vyukov's mpmc blocking queue.
//
// This queue is fast, beating throughput of a go channels with high cores at
// the expense of extra CPU.
//
// Queue's are forced to a multiplier-of-two size before returning. If
// enqueueing or dequeueing fails, enqueuers or dequeuers need to backoff
// before attempting enqueueing or dequeueing again. Failing to do so may lead
// to live locks if enqueueing or dequeueing is not be preempted by the go
// scheduler.
package mpmcdvq

import (
	"unsafe"

	"github.com/twmb/dash/primitive"
)

const cellSz = unsafe.Sizeof(cell{})

// cell is an individual spot in our queue.
type cell struct {
	// seq is a number that has a base value of its position in the queue.
	// seq changes as follows, with the base value as `b` and the size of
	// the queue as `s`:
	//
	//                value
	//     enqueue:     b+1
	//     dequeue:     b+s
	//     enqueue:   b+s+1
	//     dequeue:   b+2*s
	//     enqueue: b+2*s+1
	//     dequeue:   b+3*s
	//     etc...
	//
	// At a high level, seq tracks the total number of enqueues, with each
	// dequeue setting the enqueue count when the enqueuer reuses the cell.
	seq uintptr
	// ptr is set to what we enqueue, and null when we dequeue.
	ptr unsafe.Pointer
	// we pad between cells so that dequeues do not share with enqueues.
	_pad [primitive.FalseShare - primitive.UpSz]byte
}

// Queue represents a multi-producer, multi-consumer, fast queue.
type Queue struct {
	// padding to ensure mask/bufPtr are not on a write modified cache
	// line when trying to read mask/bufPtr.
	_pad0 [primitive.FalseShare - primitive.UpSz]byte
	// mask is the size of our queue - 1. Because the size of the queue is
	// forced to be a power of 2, we index into slots via masking.
	mask  uintptr
	cells []cell
	_pad1 [primitive.FalseShare - primitive.UpSz]byte
	// padding enqPos to not share cache lines, enqPos tracks the current
	// enqueueing position.
	enqPos uintptr
	_pad2  [primitive.FalseShare - primitive.UpSz]byte
	// padding deqPos to not share cache lines, deqPos tracks the current
	// dequeueing position.
	deqPos uintptr
	// pad at the end to not share this queue with the next variable.
	_pad3 [primitive.FalseShare - primitive.UpSz]byte
}

// New returns a new Queue, with size rounded up to the next power of 2.
func New(size uint) *Queue {
	size2 := primitive.Next2(uintptr(size))
	cells := make([]cell, size2+1) // pad one cell at the start to avoid sharing it
	for i := uintptr(0); i < size2+1; i++ {
		cells[i].seq = i - 1 // remove the pad cell from the sequence number
	}

	q := &Queue{
		mask:  size2 - 1,
		cells: cells[1:],
	}
	return q
}
