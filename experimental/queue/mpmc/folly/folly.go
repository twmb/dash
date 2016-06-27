// This transliterates Facebook's folly's MPMCQueue, which is licensed with
// Apache License, Version 2.0.

// Package follyq provides a blocking multi-producer multi-consumer queue based
// off Facebook's folly's MPMCQueue.
package follyq

import (
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/twmb/dash/futex"
	"github.com/twmb/dash/primitive"
)

const (
	upSz      = unsafe.Sizeof(uintptr(0))
	cellSz    = unsafe.Sizeof(cell{})
	cacheLine = 64

	updateSpinFreqShift = 7
)

var null unsafe.Pointer

type cell struct {
	tb   turnBroker
	ptr  unsafe.Pointer
	_pad [cacheLine - unsafe.Sizeof(turnBroker{}) + upSz]byte
}

func (c *cell) mayEnqueue(turn uintptr) bool {
	return c.tb.isTurn(turn * 2)
}

func (c *cell) mayDequeue(turn uintptr) bool {
	return c.tb.isTurn(turn*2 + 1)
}

func (c *cell) enqueue(turn uintptr, ptr unsafe.Pointer, spinCutoff *uint32, maybeUpdateSpin bool) {
	c.tb.waitFor(turn*2, spinCutoff, maybeUpdateSpin)
	c.ptr = ptr
	c.tb.completeTurn(turn * 2)
}

func (c *cell) dequeue(turn uintptr, spinCutoff *uint32, maybeUpdateSpin bool) (ptr unsafe.Pointer) {
	c.tb.waitFor(turn*2+1, spinCutoff, maybeUpdateSpin)
	ptr = c.ptr
	c.ptr = null
	c.tb.completeTurn(turn*2 + 1)
	return
}

type Queue struct {
	_pad0 [cacheLine]byte
	// lgsz is the log2(size) of our queue. This is used for quick mapping
	// a ticket into a turn for a cell.
	lgsz uintptr
	// mask is the size of our queue - 1. Because the size of the queue is
	// forced to be a power of 2, we index into cellsPtr via masking.
	mask uintptr
	// cellsPtr points to the backing array of a slice of cells. We point
	// directly to the array to have easier cell access semantics without
	// fear of shadow copies.
	cellsPtr unsafe.Pointer
	_pad1    [cacheLine - 3*upSz]byte
	// pushTicket is used by enqueuers when receiving tickets.
	pushTicket uintptr
	_pad2      [cacheLine - upSz]byte
	// popTicket is used by dequeuers when receiving tickets.
	popTicket uintptr
	_pad3     [cacheLine - upSz]byte
	// pushSpinCutoff is used to control spinning when enqueueing.
	pushSpinCutoff uint32
	_pad4          [cacheLine - 4]byte
	// popSpinCutoff is used to control spinning when dequeueing.
	popSpinCutoff uint32
	_pad5         [cacheLine - 4]byte
}

// next2 rounds up to the next power of 2 by setting all bits at or under the
// current value to 1, and then adding 1 to pow2 up.
func next2(v uintptr) uintptr {
	v--
	for i := uintptr(1); i < upSz<<3; i <<= 1 {
		v |= v >> i
	}
	return v + 1
}

func New(size uint) *Queue {
	size2 := next2(uintptr(size))
	lgsz := uintptr(0)
	for i := uintptr(1); i < size2; i <<= 1 {
		lgsz++
	}
	cells := make([]cell, 0, size2)
	for i := uintptr(0); i < size2; i++ {
		cells = append(cells, cell{tb: turnBroker{f: futex.New()}})
	}

	q := &Queue{
		lgsz:     lgsz,
		mask:     size2 - 1,
		cellsPtr: unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(&cells)).Data),
	}
	return q
}

func (q *Queue) TryEnqueue(ptr unsafe.Pointer) (enqueued bool) {
	var ticket uintptr
	ticket, enqueued = q.tryGetPushTicket()
	if enqueued {
		q.enqueue(ticket, ptr)
	}
	return
}

func (q *Queue) Enqueue(ptr unsafe.Pointer) {
	nextTicket := atomic.AddUintptr(&q.pushTicket, 1)
	q.enqueue(nextTicket-1, ptr)
}

func (q *Queue) TryDequeue() (ptr unsafe.Pointer, dequeued bool) {
	var ticket uintptr
	ticket, dequeued = q.tryGetPopTicket()
	if dequeued {
		ptr = q.dequeue(ticket)
	}
	return
}

func (q *Queue) Dequeue() (ptr unsafe.Pointer) {
	nextTicket := atomic.AddUintptr(&q.popTicket, 1)
	return q.dequeue(nextTicket - 1)
}

// tryGetPushTicket tries to obtain a push ticket for which an enqueue will not
// block.
func (q *Queue) tryGetPushTicket() (uintptr, bool) {
	curPush := atomic.LoadUintptr(&q.pushTicket)
	for {
		c := (*cell)(unsafe.Pointer(uintptr(q.cellsPtr) + (cellSz * (curPush & q.mask))))
		if !c.mayEnqueue(curPush >> q.lgsz) {
			// If we call enqueue with curPush right now, it will
			// block, but our curPush may be out of date. We can
			// increase tryGetPushTicket under contention by
			// rechecking before failing.
			prev := curPush
			curPush = atomic.LoadUintptr(&q.pushTicket)
			if prev == curPush {
				// We checked and failed twice. We cannot
				// enqueue.
				return 0, false
			}
		} else {
			// We _think_ we have a push ticket that can enqueue,
			// but now swap this local state in to claim our spot.
			var swapped bool
			curPush, swapped = primitive.CompareAndSwapUintptr(&q.pushTicket, curPush, curPush+1)
			if swapped {
				return curPush, true
			}
		}
	}
}

// tryGetPopTicket is analogous to tryGetPushTicket.
func (q *Queue) tryGetPopTicket() (uintptr, bool) {
	curPop := atomic.LoadUintptr(&q.popTicket)
	for {
		c := (*cell)(unsafe.Pointer(uintptr(q.cellsPtr) + (cellSz * (curPop & q.mask))))
		if !c.mayDequeue(curPop >> q.lgsz) {
			prev := curPop
			curPop = atomic.LoadUintptr(&q.popTicket)
			if prev == curPop {
				return 0, false
			}
		} else {
			var swapped bool
			curPop, swapped = primitive.CompareAndSwapUintptr(&q.popTicket, curPop, curPop+1)
			if swapped {
				return curPop, true
			}
		}
	}
}

// enqueue enqueues to the cell owning this ticket.
func (q *Queue) enqueue(ticket uintptr, ptr unsafe.Pointer) {
	c := (*cell)(unsafe.Pointer(uintptr(q.cellsPtr) + (cellSz * (ticket & q.mask))))
	turn := ticket >> q.lgsz
	maybeUpdateSpin := ticket>>updateSpinFreqShift == 0
	c.enqueue(turn, ptr, &q.pushSpinCutoff, maybeUpdateSpin)
}

// dequeue dequeues from the cell owning this ticket.
func (q *Queue) dequeue(ticket uintptr) (ptr unsafe.Pointer) {
	c := (*cell)(unsafe.Pointer(uintptr(q.cellsPtr) + (cellSz * (ticket & q.mask))))
	turn := ticket >> q.lgsz
	maybeUpdateSpin := ticket>>updateSpinFreqShift == 0
	return c.dequeue(turn, &q.popSpinCutoff, maybeUpdateSpin)
}
