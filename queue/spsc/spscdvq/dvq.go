package spscdvq

import (
	"sync/atomic"
	"unsafe"

	"github.com/twmb/dash/primitive"
)

// See mpmc's mpmcdvq for full comments. This code is that mpmc, whittled down
// assuming there is max one enqueue concurrent with one dequeue.

// TryEnqueue adds a value to our queue. TryEnqueue takes an unsafe.Pointer to
// avoid the necessity of wrapping a heap allocated value in an interface,
// which also goes on the heap. If the queue is full, this will return failure.
func (q *Queue) TryEnqueue(ptr unsafe.Pointer) (enqueued bool) {
	c := &q.cells[q.enqPos&q.mask]
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
	c := &q.cells[q.deqPos&q.mask]
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
