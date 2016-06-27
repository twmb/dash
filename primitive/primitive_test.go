package primitive

import "testing"

func TestCompareAndSwapUintptr(t *testing.T) {
	var addr uintptr
	fresh, swapped := CompareAndSwapUintptr(&addr, 0, 2)
	if fresh != 2 || !swapped {
		t.Errorf("got %d (swapped %v), expected %d (swapped %v) from CAS of %d-value with %d to %d", fresh, swapped, 2, true, 0, 0, 2)
	}
	fresh, swapped = CompareAndSwapUintptr(&addr, 1, 3)
	if fresh != 2 || swapped {
		t.Errorf("got %d (swapped %v), expected %d (swapped %v) from CAS of %d-value with %d to %d", fresh, swapped, 2, false, 2, 1, 3)
	}
}
