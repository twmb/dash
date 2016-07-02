// Package block provides a fast blocking primitive to wrap around code that
// may fail.
//
// Block provides blocking for semantics to surround spinning algorithms. If an
// algorithm can fail fast and can be attempted again for success, it needs
// wrapped in a block to avoid continual spinning.
//
// The general flow for use of a block is
//
//         // goroutine 1
//         for {
//                 did := lf.Sub()
//                 if did {
//                         break
//                 }
//                 var primer uint32
//                 var primed bool
//                 for !primed && !did  {
//                         primer, primed := block.Prime()
//                         did = lf.Sub()
//                 }
//                 if did {
//                         if primed {
//                                 block.Cancel()
//                         }
//                         break
//                 }
//                 block.Wait(primer)
//         }
//
//         // goroutine 2
//         lf.Pub()
//         block.Signal()
//
// Block has many internal checks to abort transitioning to a blocking state.
// Block assumes that transitioning to a blocking state is worse than spinning,
// and will not wait if it thinks forward progress may be possible. This means
// that a Block will only truly fall into waiting when absolutely nothing is
// happening in the code it is meant to provide blocking for.
//
// Because of this forward-progress assumption, Block causes quite a bit of
// unnecessary spinning. This will eat free CPU. Blocks are meant to be used
// when high throughput is desired at the expense of CPU. Blocks should only be
// used in cases where the alternative is to spin or where you want something
// faster than a mutex/condition variable combo.
package block

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/twmb/dash/primitive"
)

// Block provides a mechanism to wait around code that may spin.
type Block struct {
	_pad0   [primitive.FalseShare - 4]byte
	waiters int32
	_pad1   [primitive.FalseShare - 4]byte
	counter uintptr
	_pad2   [primitive.FalseShare - primitive.UpSz]byte
	lock    lock
	cond    *sync.Cond
	_pad3   [primitive.FalseShare - primitive.UpSz]byte
}

// New returns a new Block.
func New() *Block {
	b := new(Block)
	b.cond = sync.NewCond(&b.lock)
	return b
}

// lock implements a spinning reader/writer lock with try lock semantics.
type lock struct {
	write uint32
	_pad  [primitive.FalseShare - 4]byte
	read  uint32
}

const highBit uint32 = 1 << 31

// TryLock sets the lock in a write state. This function allows one pending
// write lock; additional pending write locks return failure.
func (l *lock) TryLock() bool {
	var write uint32
	for {
		// Add our lock desire, checking the state in the process.
		write = atomic.AddUint32(&l.write, 1)
		if write&highBit == 0 {
			break
		}
		// If the high bit is set, the lock is being unlocked. We retry
		// as we may now either be the first lock or the pending lock.
		runtime.Gosched()
	}

	switch write {
	case 1:
		// We were the first to grab this lock - signal readers to exit
		// and wait for them.
		read := atomic.AddUint32(&l.read, highBit)
		for read != highBit {
			runtime.Gosched()
			read = atomic.LoadUint32(&l.read)
		}
		return true
	case 2:
		// We were the second to grab this lock - we wait for the high
		// bit to be set when unlocking. The unlocker will see multiple
		// lock grabs and not reset the lock fully.
		for write&highBit == 0 {
			runtime.Gosched()
			write = atomic.LoadUint32(&l.write)
		}
		// We have seen the high bit - set the lock back to the locked
		// state.
		atomic.StoreUint32(&l.write, 1)
		return true
	}
	// We are not the first locker, nor the pending locker, and we did not
	// see a lock unlocking. We can return.
	return false
}

// WUnlock relinquishes our write lock.
func (l *lock) WUnlock() {
	// If more writers attempted our lock, write&^highBit will be >1, and
	// the try that got 2 will be waiting in pending state. That waiter
	// will now see we have unlocked, so we can leave.
	write := atomic.AddUint32(&l.write, highBit)
	if write&^highBit > 1 {
		return
	}
	// If nobody else attempted the lock by the time added the high bit,
	// we must let readers continue (first) and then reset our write lock.
	atomic.StoreUint32(&l.read, 0)
	atomic.StoreUint32(&l.write, 0)
}

// Lock, a noop, is provided to implement the sync.Locker interface for a
// sync.Cond. In reality, after being woken from a Wait, we call TryRLock
// ourselves.
func (l *lock) Lock() {
}

// TryRLock attempts to grab a reader lock, failing if a writer has locked.
func (l *lock) TryRLock() bool {
	var swapped bool
	read := atomic.LoadUint32(&l.read)
	for {
		if read&highBit != 0 { // writer has grabbed lock
			return false
		}
		read, swapped = primitive.CompareAndSwapUint32(&l.read, read, read+1)
		if swapped { // we got a read lock
			return true
		}
	}
}

// Unlock decrements the lock by one reader. This is the counterpart to
// TryRLock (if it succeeds), but named "Unlock" to satisfy the sync.Locker
// interface.
func (l *lock) Unlock() {
	atomic.AddUint32(&l.read, math.MaxUint32) // wrap to decrement by one
}

// Prime, called before a function that may fail, returns what you will call
// Wait with. If you do not call wait, you must call Cancel.
func (b *Block) Prime(last uintptr) (primer uintptr, primed bool) {
	primer = atomic.LoadUintptr(&b.counter)
	if primer != last {
		return
	}
	runtime.Gosched()
	primer = atomic.LoadUintptr(&b.counter)
	if primer != last || atomic.LoadUint32(&b.lock.write) != 0 {
		return
	}
	primed = true
	atomic.AddInt32(&b.waiters, 1)
	return
}

// Cancel, which must be called if not calling Wait after a Prime, cancels one
// primed block call.
func (b *Block) Cancel() {
	atomic.AddInt32(&b.waiters, -1)
}

// Wait blocks until the block has been signaled to continue. This may
// spuriously return early. The assumption is that re-checking an operation
// that may fail is cheaper than blocking.
func (b *Block) Wait(primer uintptr) {
	for {
		for {
			runtime.Gosched()
			if primer != atomic.LoadUintptr(&b.counter) {
				atomic.AddInt32(&b.waiters, -1)
				return

			}
			if b.lock.TryRLock() {
				break
			}
		}
		if primer != b.counter {
			atomic.AddInt32(&b.waiters, -1)
			b.lock.Unlock()
			return
		}
		b.cond.Wait()
		// Waking up does not grab any lock.
	}
}

// Signal, to be called after every operation that can un-wait a block, awakens
// all block waiters.
func (b *Block) Signal() {
	if atomic.LoadInt32(&b.waiters) == 0 {
		return
	}
	// We either get the lock, wait in pending state until we get the lock,
	// or return because somebody else is in a pending state.
	//
	// The logic for having only _one_ pending wait is as follows:
	//
	// - Prime calls can observe the counter either before an increment or
	//   directly after an increment. We must keep a pending signal because
	//   a prime call observed after an active increment will return on
	//   Wait from the pending signal.
	//
	// - We need only need one pending signal because all active prime
	//   calls will, at worst, observe the actively signaling counter. That
	//   counter will be incremented by the pending singal.
	//
	// - All future signals can be collapsed into one pending signal which
	//   will wake everything that race read the actively signaling counter.
	//
	// - One pending signal is the same as pathologically racing all
	//   signals in front of any active Prime calls continuing to their
	//   Wait. That is, having one pending singal is the _worst case_ of
	//   processing all signals consecutively.
	//
	// In summary, anything that called Prime by _now_ cares about either
	// this signal or one pending signal. Eliding _all_ signals into one
	// pending signal is the _same_ as having all signals race finishing
	// immediately before any future Prime call, which would be the worst
	// case scenario from a signaling perspective.
	if !b.lock.TryLock() {
		return
	}
	atomic.AddUintptr(&b.counter, 1)
	b.cond.Broadcast()
	b.lock.WUnlock()
}
