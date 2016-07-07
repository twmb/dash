package mpscdvq

import (
	"sync/atomic"
	"unsafe"

	"github.com/twmb/dash/primitive"
)

// See mpmc's mpmcdvq for full comments. This code is that mpmc, whittled down
// assuming there are many enqueuers concurrent with on dequeue.

// TryEnqueue adds a value to our queue. TryEnqueue takes an unsafe.Pointer to
// avoid the necessity of wrapping a heap allocated value in an interface,
// which also goes on the heap. If the queue is full, this will return failure.
func (q *Queue) TryEnqueue(ptr unsafe.Pointer) (enqueued bool) {
	var c *cell
	pos := atomic.LoadUintptr(&q.enqPos)
	for {
		c = (*cell)(unsafe.Pointer(uintptr(q.bufPtr) + (cellSz * (pos & q.mask))))
		seq := atomic.LoadUintptr(&c.seq)
		cmp := int(seq - pos)
		if cmp == 0 {
			var swapped bool
			if pos, swapped = primitive.CompareAndSwapUintptr(&q.enqPos, pos, pos+1); swapped {
				enqueued = true
				break
			}
			continue
		}
		if cmp < 0 {
			return
		}
		pos = atomic.LoadUintptr(&q.enqPos)
	}
	c.ptr = ptr
	atomic.StoreUintptr(&c.seq, pos)
	return
}

// TryDequeue dequeues a value from our queue. If the queue is empty, this
// will return failure.
func (q *Queue) TryDequeue() (ptr unsafe.Pointer, dequeued bool) {
	c := (*cell)(unsafe.Pointer(uintptr(q.bufPtr) + (cellSz * (q.deqPos & q.mask))))
	seq := atomic.LoadUintptr(&c.seq)
	if seq < q.deqPos+1 {
		return
	}
	q.deqPos++
	ptr = c.ptr
	c.ptr = primitive.Null
	atomic.StoreUintptr(&c.seq, q.deqPos+q.mask)
	return ptr, true
}
