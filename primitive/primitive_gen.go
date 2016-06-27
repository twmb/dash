// +build race

package primitive

import "sync/atomic"

// CompareAndSwapUintptr executes the compare-and-swap operation for a uintptr
// value, returning the freshest addr value after execution and whether the
// CAS succeeded.
func CompareAndSwapUintptr(addr *uintptr, old, new uintptr) (fresh uintptr, swapped bool) {
	swapped = atomic.CompareAndSwapUintptr(addr, old, new)
	if swapped {
		fresh = new
	} else {
		fresh = atomic.LoadUintptr(addr)
	}
	return
}

// CompareAndSwapInt64 executes the compare-and-swap operation for a int64
// value, returning the freshest addr value after execution and whether the CAS
// succeeded.
func CompareAndSwapInt64(addr *int64, old, new int64) (fresh int64, swapped bool) {
	swapped = atomic.CompareAndSwapInt64(addr, old, new)
	if swapped {
		fresh = new
	} else {
		fresh = atomic.LoadInt64(addr)
	}
	return
}

// CompareAndSwapUint64 executes the compare-and-swap operation for a uint64
// value, returning the freshest addr value after execution and whether the CAS
// succeeded.
func CompareAndSwapUint64(addr *uint64, old, new uint64) (fresh uint64, swapped bool) {
	swapped = atomic.CompareAndSwapUint64(addr, old, new)
	if swapped {
		fresh = new
	} else {
		fresh = atomic.LoadUint64(addr)
	}
	return
}

// CompareAndSwapInt32 executes the compare-and-swap operation for an int32
// value, returning the freshest addr value after execution and whether the CAS
// succeeded.
func CompareAndSwapInt32(addr *int32, old, new int32) (fresh int32, swapped bool) {
	swapped = atomic.CompareAndSwapInt32(addr, old, new)
	if swapped {
		fresh = new
	} else {
		fresh = atomic.LoadInt32(addr)
	}
	return
}

// CompareAndSwapUint32 executes the compare-and-swap operation for a uint32
// value, returning the freshest addr value after execution and whether the CAS
// succeeded.
func CompareAndSwapUint32(addr *uint32, old, new uint32) (fresh uint32, swapped bool) {
	swapped = atomic.CompareAndSwapUint32(addr, old, new)
	if swapped {
		fresh = new
	} else {
		fresh = atomic.LoadUint32(addr)
	}
	return
}
