// This transliterates Facebook's folly's Futex source, which is licensed with
// Apache License, Version 2.0.

// Package futex provides a locking structure that avoids spurious wake-ups.
//
// Unfortunately, since we cannot control Go allocation, waiting must heap
// allocate. The performance of this futex is subpar and needs tested more;
// currently, only an emulated futex is provided and system futexes are not
// used if available.
package futex

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// Futex provides a locking structure that avoids spurious wake-ups. Waiting
// is performed on an expected state; if the state has changed before the wait,
// it does not wait.
type Futex struct {
	State    uintptr
	origAddr uintptr
	bucket   synthBucket
}

func New() *Futex {
	f := new(Futex)
	f.origAddr = uintptr(unsafe.Pointer(&f))
	f.bucket = buckets[twhash(uint64(f.origAddr))%numBuckets]
	return f
}

type Result int

const (
	// ValueChanged is returned in Wait when Futex state had changed by the
	// time Wait was called.
	ValueChanged Result = iota
	// Awoken is returned in Wait when signalled via a futex wake call.
	Awoken
)

// Code below provides structures to emulate a system futex.

type synthNode struct {
	next *synthNode
	prev *synthNode

	addr      uintptr
	waitMask  uintptr
	signalled bool
	mtx       *sync.Mutex
	cond      *sync.Cond
}

type synthBucket struct {
	mtx   *sync.Mutex
	nodes *synthNode // nodes _is_ the root
}

const numBuckets = 4096

var buckets []synthBucket

func init() {
	buckets = make([]synthBucket, 0, numBuckets)
	for i := 0; i < numBuckets; i++ {
		sentinel := new(synthNode)
		sentinel.next = sentinel
		sentinel.prev = sentinel
		buckets = append(buckets, synthBucket{mtx: new(sync.Mutex), nodes: sentinel})
	}
}

func twhash(addr uint64) uint64 {
	addr = (^addr) + (addr << 21) // addr *= (1 << 21) - 1; addr -= 1;
	addr = addr ^ (addr >> 24)
	addr = addr + (addr << 3) + (addr << 8) // addr *= 1 + (1 << 3) + (1 << 8)
	addr = addr ^ (addr >> 14)
	addr = addr + (addr << 2) + (addr << 4) // addr *= 1 + (1 << 2) + (1 << 4)
	addr = addr ^ (addr >> 28)
	addr = addr + (addr << 31) // addr *= 1 + (1 << 31)
	return addr
}

// Wake wakes count waiters that and with the given waitMask.
func (f *Futex) Wake(count uint32, waitMask uintptr) uint32 {
	f.bucket.mtx.Lock()

	numAwoken := uint32(0)
	sentinel := f.bucket.nodes
	for iter := sentinel.next; numAwoken < count && iter != sentinel; iter = iter.next {
		if iter.addr == f.origAddr && iter.waitMask&waitMask != 0 {
			numAwoken++

			// unlink
			iter.prev.next = iter.next
			iter.next.prev = iter.prev

			// Grab the lock to ensure we are either before waiting
			// (before checking signal), or actively waiting (will
			// check signal).
			iter.mtx.Lock()
			iter.signalled = true
			iter.cond.Signal()
			iter.mtx.Unlock()
		}
	}

	f.bucket.mtx.Unlock()

	return numAwoken
}

// Wait takes an expected state to wait for and a mask if we need to wait.
// Masking allows us to selectively wake up multiple callers based on their
// chosen mask. waitMask must not be zero.
func (f *Futex) Wait(expectState uintptr, waitMask uintptr) Result {
	// Everything here should be stack allocated, but alas... Go.
	var nodeMtx sync.Mutex
	node := synthNode{
		addr:     f.origAddr,
		waitMask: waitMask,
		mtx:      &nodeMtx,
	}
	node.cond = sync.NewCond(node.mtx)

	// Lock before enqueueing - if the state changed, we are about to wake.
	// We do not want to miss that wake signal here. Thus, we block the
	// wake.
	//
	// Either we will see the state change not not even enqueue ourselves
	// to wait, _or_ we will miss the state change but observe the wake.
	f.bucket.mtx.Lock()
	if atomic.LoadUintptr(&f.State) != expectState {
		f.bucket.mtx.Unlock()
		return ValueChanged
	}
	node.prev = f.bucket.nodes.prev
	f.bucket.nodes.prev.next = &node
	f.bucket.nodes.prev = &node
	node.next = f.bucket.nodes
	f.bucket.mtx.Unlock()

	// Wait to be signalled.
	node.mtx.Lock()
	for !node.signalled {
		node.cond.Wait()
	}
	node.mtx.Unlock()

	return Awoken
}
