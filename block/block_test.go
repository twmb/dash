package block

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These benchmarks demonstrate simple throughput differences of my lock vs.
// sync.RWMutex, where my lock is marginally different, and then very contended
// differences, where my lock fast track wlock returns.  In "very contended"
// (i.e., anything with no number), benchmarks, we start 10 goroutines per proc.
//
// As it turns out, my lock is worst for almost any standard usage of a lock.
// The main benefit comes from readers being _unable_ to get the lock if one
// writer is in a pending state. Readers have no chance to exclude writers
// during the pending transition. This is beneficial for a block.
//
// I have tried separating out the writer bits into a separate mutex. It is
// much slower in _use_, even though benchmarks show better numbers for
// directly grabbing the lock.

func TestLock(t *testing.T) {
	var l lock
	locked := l.TryLock()
	if !locked {
		t.Error("expected locked after TryLock")
	}
	go l.TryLock()
	time.Sleep(time.Millisecond)
	if atomic.LoadUint32(&l.write) != 2 {
		t.Errorf("expected pendingWait after second TryLock")
	}
	if l.TryLock() {
		t.Error("unexpected TryLock get on doubly locked lock")
	}
	l.WUnlock()
	time.Sleep(time.Millisecond)
	if atomic.LoadUint32(&l.write) != 1 {
		t.Error("expected locked after unlock from pendingWait")
	}

}

// wlock/wunlock

func BenchmarkLockW1(b *testing.B) {
	var l lock
	for i := 0; i < b.N; i++ {
		if !l.TryLock() {
			b.Fatal("unable to lock")
		}
		l.WUnlock()
	}
}

func BenchmarkRWMutexW1(b *testing.B) {
	var mtx sync.RWMutex
	for i := 0; i < b.N; i++ {
		mtx.Lock()
		mtx.Unlock()
	}
}

// rlock/runlock

func BenchmarkLockR1(b *testing.B) {
	var l lock
	for i := 0; i < b.N; i++ {
		if !l.TryRLock() {
			b.Fatal("unable to lock")
		}
		l.Unlock()
	}
}

func BenchmarkRWMutexR1(b *testing.B) {
	var mtx sync.RWMutex
	for i := 0; i < b.N; i++ {
		mtx.RLock()
		mtx.RUnlock()
	}
}

// contended (two only) wlock/wunlock only two because my lock fast track
// returns on more than two

func BenchmarkLockW2(b *testing.B) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(2))
	var l lock
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !l.TryLock() {
				panic(fmt.Sprintf("%b", l.write))
			}
			l.WUnlock()
		}
	})
}

func BenchmarkRWMutexW2(b *testing.B) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(2))
	var mtx sync.RWMutex
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mtx.Lock()
			mtx.Unlock()
		}
	})
}

// very contended rlock/runlock (10 goroutines per proc).

func BenchmarkLockR(b *testing.B) {
	b.SetParallelism(10) // 10 goroutines per proc
	var l lock
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !l.TryRLock() {
				panic(fmt.Sprintf("%b", l.write))
			}
			l.Unlock()
		}
	})
}

func BenchmarkRWMutexR(b *testing.B) {
	b.SetParallelism(10)
	var mtx sync.RWMutex
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mtx.RLock()
			mtx.RUnlock()
		}
	})
}

// very contended wlock/wunlock

func BenchmarkLockW(b *testing.B) {
	b.SetParallelism(10)
	var l lock
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if l.TryLock() {
				l.WUnlock()
			}
		}
	})
}

func BenchmarkRWMutexW(b *testing.B) {
	b.SetParallelism(10)
	var mtx sync.RWMutex
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mtx.Lock()
			mtx.Unlock()
		}
	})
}

// very contended competing wlock/wunlock and rlock/unlock

func BenchmarkLockRW(b *testing.B) {
	b.SetParallelism(10)
	var l lock
	b.RunParallel(func(pb *testing.PB) {
		full := true
		for pb.Next() {
			if full {
				if l.TryLock() {
					l.WUnlock()
				}
			} else {
				if l.TryRLock() {
					l.Unlock()
				}
			}
			full = !full
		}
	})
}

func BenchmarkRWMutexRW(b *testing.B) {
	b.SetParallelism(10)
	var mtx sync.RWMutex
	b.RunParallel(func(pb *testing.PB) {
		full := true
		for pb.Next() {
			if full {
				mtx.Lock()
				mtx.Unlock()
			} else {
				mtx.RLock()
				mtx.RUnlock()
			}
			full = !full
		}
	})
}

// contended block, and the mutex/cond obvious implementation beneath

func BenchmarkBlock(b *testing.B) {
	bl := New()
	die := make(chan struct{})
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go func() {
			for {
				select {
				case <-die:
					return
				default:
					// Gosched can double as our item of work.
					runtime.Gosched()
				}
				bl.Signal()
			}
		}()
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			primer, primed := bl.Prime(0)
			if !primed {
				continue
			}
			bl.Wait(primer)
		}
	})
	close(die)
}

func BenchmarkMtxBlock(b *testing.B) {
	var primer int64
	m := new(sync.RWMutex)
	c := sync.NewCond(m.RLocker())
	die := make(chan struct{})
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go func() {
			for {
				select {
				case <-die:
					return
				default:
					// Gosched doubles as our item of work, as above.
					runtime.Gosched()
				}
				m.Lock()
				atomic.AddInt64(&primer, 1)
				c.Broadcast()
				m.Unlock()
			}
		}()
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			myPrimer := atomic.LoadInt64(&primer)
			for {
				m.RLock()
				if myPrimer != primer {
					m.RUnlock()
					break
				}
				c.Wait()
				m.RUnlock()
			}
		}
	})
	close(die)
}
