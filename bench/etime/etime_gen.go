// +build !amd64

// Package etime provides extended time primitives for systems where available.
package etime

import "time"

// Now returns the current nanosecond.
func Now() int64 {
	return time.Now().UnixNano()
}

// Duration returns the time duration between two etime.Now calls, ignoring the
// second argument.
func Duration(delta, _ int64) time.Duration {
	return time.Duration(delta)
}
