package spmcdvq

import (
	"sync/atomic"
	"unsafe"

	"github.com/twmb/dash/primitive"
)

// See mpmc's mpmcdvq for full comments. This code is that mpmc, whittled down
// assuming there are is one enqueuer concurrent with many dequeuers.

// TryEnqueue adds a value to our queue. TryEnqueue takes an unsafe.Pointer to
// avoid the necessity of wrapping a heap allocated value in an interface,
// which also goes on the heap. If the queue is full, this will return failure.
func (q *Queue) TryEnqueue(ptr unsafe.Pointer) (enqueued bool) {
	c := (*cell)(unsafe.Pointer(uintptr(q.bufPtr) + (cellSz * (q.enqPos & q.mask))))
	seq := atomic.LoadUintptr(&c.seq)
	if seq < q.enqPos {
		return
	}
	q.enqPos++
	c.ptr = ptr
	atomic.StoreUintptr(&c.seq, q.enqPos)
	return true
}

// TryDequeue dequeues a value from our queue. If the queue is empty, this
// will return failure.
func (q *Queue) TryDequeue() (ptr unsafe.Pointer, dequeued bool) {
	var c *cell
	pos := atomic.LoadUintptr(&q.deqPos)
	for {
		c = (*cell)(unsafe.Pointer(uintptr(q.bufPtr) + (cellSz * (pos & q.mask))))
		seq := atomic.LoadUintptr(&c.seq)
		cmp := int(seq - (pos + 1))
		if cmp == 0 {
			var swapped bool
			if pos, swapped = primitive.CompareAndSwapUintptr(&q.deqPos, pos, pos+1); swapped {
				dequeued = true
				break
			}
			continue
		}
		if cmp < 0 {
			return
		}
		pos = atomic.LoadUintptr(&q.deqPos)
	}
	ptr = c.ptr
	c.ptr = primitive.Null
	atomic.StoreUintptr(&c.seq, pos+q.mask)
	return
}
