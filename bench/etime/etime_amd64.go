// +build amd64

// Package etime provides extended time primitives for systems where available.
package etime

import "time"

// Now returns the current time stamp counter for a cpu. This should only be
// used if the TSC is invariant.
func Now() int64

// Duration returns the actual time between two etime.Now calls, given the cpu
// clock rate. For example, if the clock rate is 2.2GHz, clock should be
// 2,200,000,000 (2.2*1e9).
func Duration(delta, clock int64) time.Duration {
	// clock is ticks / second
	// delta is ticks
	// 1e9 * delta / clock is nanoseconds
	return time.Duration(1e9 * float64(delta) / float64(clock))
}
