package mpmcdvq

import (
	"sync/atomic"
	"unsafe"

	"github.com/twmb/dash/primitive"
)

// TryEnqueue adds a value to our queue. TryEnqueue takes an unsafe.Pointer to
// avoid the necessity of wrapping a heap allocated value in an interface,
// which also goes on the heap. If the queue is full, this will return failure.
func (q *Queue) TryEnqueue(ptr unsafe.Pointer) (enqueued bool) {
	var c *cell
	// Race load our enqPos,
	pos := atomic.LoadUintptr(&q.enqPos)
	for {
		// load the cell at that enqPos,
		c = (*cell)(unsafe.Pointer(uintptr(q.bufPtr) + (cellSz * (pos & q.mask))))
		// load the sequence number in that cell,
		seq := atomic.LoadUintptr(&c.seq)
		// and, if the sequence number is (enqPos), we have a spot to
		// enqueue into.
		cmp := int(seq - pos)
		if cmp == 0 {
			var swapped bool
			// Try to claim the enqPos to ourselves to enqueue,
			// updating pos to the new value.
			if pos, swapped = primitive.CompareAndSwapUintptr(&q.enqPos, pos, pos+1); swapped {
				enqueued = true
				break
			}
			continue
		}
		if cmp < 0 {
			// If the sequence number was less than enqPos, the
			// queue is full.
			return
		}
		// If the sequence number was larger than enqPos,
		// somebody else just updated the sequence number and
		// our loaded enqPos is out of date.
		pos = atomic.LoadUintptr(&q.enqPos)
	}
	// We have won the race and can enqueue - set the pointer.
	c.ptr = ptr
	// Update the cell's sequence number for dequeueing.
	atomic.StoreUintptr(&c.seq, pos)
	return
}

// TryDequeue dequeues a value from our queue. If the queue is empty, this
// will return failure.
func (q *Queue) TryDequeue() (ptr unsafe.Pointer, dequeued bool) {
	var c *cell
	// Race load our deqPos,
	pos := atomic.LoadUintptr(&q.deqPos)
	for {
		// load the cell at that deqPos,
		c = (*cell)(unsafe.Pointer(uintptr(q.bufPtr) + (cellSz * (pos & q.mask))))
		// load the sequence number in that cell,
		seq := atomic.LoadUintptr(&c.seq)
		// and, if the sequence number is (deqPos + 1), we have an
		// enqueued value to dequeue.
		cmp := int(seq - (pos + 1))
		if cmp == 0 {
			var swapped bool
			// Try to claim the deqPos to ourselves to dequeue,
			// updating pos to the new value.
			if pos, swapped = primitive.CompareAndSwapUintptr(&q.deqPos, pos, pos+1); swapped {
				dequeued = true
				break
			}
			continue
		}
		if cmp < 0 {
			// If the sequence number was less than deqPos + 1,
			// the queue is empty.
			return
		}
		// If the sequence number was larger than (deqPos+1),
		// somebody else just updated the sequence number and
		// our loaded deqPos is out of date.
		pos = atomic.LoadUintptr(&q.deqPos)
	}
	// We have won the race and can dequeue - grab the pointer.
	ptr = c.ptr
	c.ptr = primitive.Null
	// Update the cell's sequence number for the next enqueue.
	atomic.StoreUintptr(&c.seq, pos+q.mask)
	return
}
