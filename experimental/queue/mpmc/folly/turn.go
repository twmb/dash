package follyq

import (
	"math"
	"sync/atomic"

	"github.com/twmb/dash/futex"
	"github.com/twmb/dash/primitive"
)

const (
	turnShift    = 6
	turnWaitMask = 1<<turnShift - 1

	// minSpins is the minimum spin count that we will update a remote spin
	// cutoff to.
	minSpins = 4
	// maxSpins is the maximum spin count that we will update a remote spin
	// cutoff to, and the count we use when probing for updating.
	maxSpins = 2000
)

type turnBroker struct {
	f *futex.Futex
}

func (t turnBroker) isTurn(turn uintptr) bool {
	return getTurnNumber(atomic.LoadUintptr(&t.f.State)) == turn
}

func (t turnBroker) waitFor(turn uintptr, spinCutoff *uint32, updateSpinCutoff bool) {
	givenSpinCount := atomic.LoadUint32(spinCutoff)
	spinCount := givenSpinCount
	if updateSpinCutoff || givenSpinCount == 0 {
		spinCount = maxSpins
	}

	var tries uint32
	state := atomic.LoadUintptr(&t.f.State)
	for ; ; tries++ {
		curTurn := getTurnNumber(state)
		if curTurn == turn {
			break
		}

		waitingFor := turn - curTurn
		if waitingFor >= math.MaxUint32>>(turnShift+1) {
			panic("turn is in the past")
		}

		if tries < spinCount {
			primitive.Pause()
			state = atomic.LoadUintptr(&t.f.State)
			continue
		}

		curWaitingFor := getTurnWait(state)

		var newState uintptr
		if waitingFor <= curWaitingFor {
			// A later turn is already being waited for - we will
			// hop on that bandwagon and wait with it.
			newState = state
		} else {
			newState = encodeTurn(curTurn, waitingFor)

			if state != newState {
				var swapped bool
				if state, swapped = primitive.CompareAndSwapUintptr(&t.f.State, state, newState); !swapped {
					continue
				}
			}
		}

		t.f.Wait(newState, futexChannel(turn))
		state = atomic.LoadUintptr(&t.f.State)
	}

	if updateSpinCutoff || givenSpinCount == 0 {
		var spinUpdate uint32
		if tries >= maxSpins {
			// If we hit maxSpins, then spinning is pointless, so
			// the right spinCutoff is the minimum possible.
			spinUpdate = minSpins
		} else {
			// To account for variations, we allow ourself to spin
			// 2*N when we think that N is actually required in
			// order to succeed.
			spinUpdate = minSpins
			dubTries := tries << 1
			if dubTries > spinUpdate {
				spinUpdate = dubTries
			}
			if maxSpins < spinUpdate {
				spinUpdate = maxSpins
			}
		}
		if givenSpinCount == 0 {
			atomic.StoreUint32(spinCutoff, spinUpdate)
		} else {
			// Per Facebook, "Exponential moving average with alpha
			// of 7/8"... k.
			spinUpdate = uint32(int(givenSpinCount) + (int(spinUpdate)-int(givenSpinCount))>>3)
			// Try once but keep moving if somebody else updated.
			atomic.CompareAndSwapUint32(spinCutoff, givenSpinCount,
				spinUpdate)
		}
	}
}

// completeTurn unblocks a thread running waitFor(turn + 1).
func (t turnBroker) completeTurn(turn uintptr) {
	state := atomic.LoadUintptr(&t.f.State)
	for {
		curWaitingFor := getTurnWait(state)
		var oneLess uintptr
		if curWaitingFor > 0 {
			oneLess = curWaitingFor - 1
		}
		newState := encodeTurn(turn+1, oneLess)
		var swapped bool
		if state, swapped = primitive.CompareAndSwapUintptr(&t.f.State, state, newState); swapped {
			if curWaitingFor != 0 {
				// We need to wake all waiters. If there is a
				// waiter for turn 0, and a wraparound waiter
				// for turn 32, and we wake only turn 32, it
				// will go back to waiting while turn 0 still
				// needs to be awoken.
				t.f.Wake(math.MaxUint32, futexChannel(turn+1))
			}
			break
		}
	}
}

func getTurnNumber(turn uintptr) uintptr {
	return (turn &^ turnWaitMask) >> turnShift
}

func getTurnWait(turn uintptr) uintptr {
	return turn & turnWaitMask
}

func encodeTurn(turnNumber uintptr, turnWait uintptr) uintptr {
	if turnWait > turnWaitMask {
		turnWait = turnWaitMask
	}
	return turnNumber<<turnShift | turnWait
}

func futexChannel(turn uintptr) uintptr {
	return 1 << (turn & ((upSz << 3) - 1))
}
