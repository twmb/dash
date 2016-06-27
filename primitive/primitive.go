// Package primitive provides low level operations implemented in Go assembly.
//
// Not all operations are available on all processors, nor all all operations
// implemented for all processors. Some of these functions borrow or copy
// directly from the Go runtime.
package primitive

import "unsafe"

const (
	// CacheLine is the number of bytes on an Intel cache line (and
	// presumably others).
	CacheLine = 64
	// FalseShare is the number of bytes in a false sharing range for CPUs.
	// Intel will prefetch a second cache line when loading a first.
	FalseShare = 128
	// UpSz is the size of a pointer on this system.
	UpSz = unsafe.Sizeof(uintptr(0))
)

// Null is a null pointer, used to remove references from values
// holding unsafe.Pointers.
var Null unsafe.Pointer

// Next2 returns v rounded up to the next power of 2.
func Next2(v uintptr) uintptr {
	v--
	for i := uintptr(1); i < UpSz<<3; i <<= 1 {
		v |= v >> i
	}
	return v + 1
}
