// +build amd64
// +build !race

package primitive

// CompareAndSwapUintptr executes the compare-and-swap operation for a uintptr
// value, returning the freshest addr value after execution and whether the
// CAS succeeded.
func CompareAndSwapUintptr(addr *uintptr, old, new uintptr) (fresh uintptr, swapped bool)

// CompareAndSwapInt64 executes the compare-and-swap operation for an int64
// value, returning the freshest addr value after execution and whether the CAS
// succeeded.
func CompareAndSwapInt64(addr *int64, old, new int64) (fresh int64, swapped bool)

// CompareAndSwapUint64 executes the compare-and-swap operation for a uint64
// value, returning the freshest addr value after execution and whether the CAS
// succeeded.
func CompareAndSwapUint64(addr *uint64, old, new uint64) (fresh uint64, swapped bool)

// CompareAndSwapInt32 executes the compare-and-swap operation for an int32
// value, returning the freshest addr value after execution and whether the CAS
// succeeded.
func CompareAndSwapInt32(addr *int32, old, new int32) (fresh int32, swapped bool)

// CompareAndSwapUint32 executes the compare-and-swap operation for a uint32
// value, returning the freshest addr value after execution and whether the CAS
// succeeded.
func CompareAndSwapUint32(addr *uint32, old, new uint32) (fresh uint32, swapped bool)
