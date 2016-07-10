package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"syscall"
	"time"
	"unsafe"

	"github.com/twmb/dash/block"
	"github.com/twmb/dash/queue/mpmc/mpmcdvq"
	"github.com/twmb/dash/queue/mpsc/mpscdvq"
	"github.com/twmb/dash/queue/spmc/spmcdvq"
	"github.com/twmb/dash/queue/spsc/spscdvq"

	"github.com/twmb/dash/bench/etime"
	"github.com/twmb/dash/bench/qbench"
)

var clock = flag.Int64("clock-rate", 2600000000, "clock rate for processors (cat /proc/cpuinfo | grep model - 2.2GHz is 2,200,000,000)")
var messages = flag.Int("messages", 1<<20, "count of messages to pass through every banchmark")

// queueSize is the size of our queue.
const queueSize = 2048

// Int64s is used to sort timings.
type Int64s []int64

func (is Int64s) Len() int           { return len(is) }
func (is Int64s) Swap(i, j int)      { is[i], is[j] = is[j], is[i] }
func (is Int64s) Less(i, j int) bool { return is[i] < is[j] }

/******************************************************************************
 * Implement qbench interfaces.                                               *
 ******************************************************************************/

// Chan is a queue for a simple built in channel.
type Chan chan unsafe.Pointer

func (ch Chan) Enqueue(enq unsafe.Pointer) {
	ch <- enq
}

func (ch Chan) Dequeue() unsafe.Pointer {
	return <-ch
}

// Define a DVQ interface so mpmc, spmc, mpsc, spsc can all use the same
// Enqueue and Dequeue functions.

type DVQ interface {
	TryEnqueue(unsafe.Pointer) bool
	TryDequeue() (unsafe.Pointer, bool)
}

// BlockDVQ adds blocking around all dvq's.
type BlockDVQ struct {
	Q    DVQ
	EnqB *block.Block
	DeqB *block.Block
}

func (q BlockDVQ) Enqueue(enq unsafe.Pointer) {
	for {
		// Attempt faster non-block-using path.
		enqueued := q.Q.TryEnqueue(enq)
		if enqueued {
			// We enqueued, signal the dequeue block.
			q.DeqB.Signal()
			break
		}
		// We were unable to dequeue; retry, but use our block
		// if we fail again.
		var primer uintptr
		var primed bool
		for !primed && !enqueued {
			primer, primed = q.EnqB.Prime(primer)
			enqueued = q.Q.TryEnqueue(enq)
		}
		if enqueued {
			if primed {
				q.EnqB.Cancel()
			}
			q.DeqB.Signal()
			break
		}
		// Failed enqueueing after priming, wait to be awoken.
		q.EnqB.Wait(primer)
	}
}

func (q BlockDVQ) Dequeue() unsafe.Pointer {
	for {
		deq, dequeued := q.Q.TryDequeue()
		if dequeued {
			q.EnqB.Signal()
			return deq
		}
		var primer uintptr
		var primed bool
		for !primed && !dequeued {
			primer, primed = q.DeqB.Prime(primer)
			deq, dequeued = q.Q.TryDequeue()
		}
		if dequeued {
			if primed {
				q.DeqB.Cancel()
			}
			q.EnqB.Signal()
			return deq
		}
		q.DeqB.Wait(primer)
	}
}

/******************************************************************************
 * Create the functions used to start benchmarks                              *
 ******************************************************************************/

func benchChan(cfg qbench.Cfg) qbench.Results {
	cfg.Impl = Chan(make(chan unsafe.Pointer, queueSize))
	return qbench.Bench(cfg)
}

func benchMpMcDVq(cfg qbench.Cfg) qbench.Results {
	cfg.Impl = BlockDVQ{
		Q:    mpmcdvq.New(queueSize),
		EnqB: block.New(),
		DeqB: block.New(),
	}
	return qbench.Bench(cfg)
}

func benchMpScDVq(cfg qbench.Cfg) qbench.Results {
	cfg.Impl = BlockDVQ{
		Q:    mpscdvq.New(queueSize),
		EnqB: block.New(),
		DeqB: block.New(),
	}
	return qbench.Bench(cfg)
}

func benchSpMcDVq(cfg qbench.Cfg) qbench.Results {
	cfg.Impl = BlockDVQ{
		Q:    spmcdvq.New(queueSize),
		EnqB: block.New(),
		DeqB: block.New(),
	}
	return qbench.Bench(cfg)
}

func benchSpScDVq(cfg qbench.Cfg) qbench.Results {
	cfg.Impl = BlockDVQ{
		Q:    spscdvq.New(queueSize),
		EnqB: block.New(),
		DeqB: block.New(),
	}
	return qbench.Bench(cfg)
}

/******************************************************************************
 * Process qbench timings.                                                    *
 ******************************************************************************/

func dur(d int64) time.Duration {
	return etime.Duration(d, *clock)
}

func avg(times []int64) time.Duration {
	sum := float64(0)
	for _, time := range times {
		sum += float64(time)
	}
	return time.Duration(sum / float64(len(times)))
}

func processResults(typ string, results qbench.Results) {
	for _, tt := range []struct {
		title   string
		timings [][]int64
	}{
		{"enq", results.EnqueueTimings},
		{"deq", results.DequeueTimings},
		{"thr", results.ThroughputTimings},
	} {
		totLen := 0
		for _, timing := range tt.timings {
			totLen += len(timing)
		}

		all := make([]int64, 0, totLen)
		for _, timing := range tt.timings {
			for _, t := range timing {
				all = append(all, t)
			}
		}
		sort.Sort(Int64s(all))

		rawMin, rawMax, rawAvg := dur(all[0]), dur(all[len(all)-1]), avg(all)
		// Trim the top 0.01% and bottom 1% to account for random system jitter.
		// Forget about safety checks, just benchmark lots of messages.
		cutLen := int64(0.0001 * float64(len(all)))
		all = all[cutLen : int64(len(all))-cutLen]
		min, q1, median, q3, max, gAvg, tot :=
			dur(all[0]),
			dur(all[len(all)/4]),
			dur(all[len(all)/2]),
			dur(all[3*len(all)/4]),
			dur(all[len(all)-1]),
			avg(all),
			dur(results.TotalTiming)

		fmt.Printf("%s rmin[%v] min[%v] q1[%v] med[%v] q3[%v] max[%v] rmax[%v] ravg[%v] avg[%v] tot[%v]\n",
			tt.title, rawMin, min, q1, median, q3, max, rawMax, rawAvg, gAvg, tot)

		fname := fmt.Sprintf("e%dd%d.%s.%s", results.Enqueuers, results.Dequeuers, tt.title, typ)
		f, err := os.OpenFile(fname, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			fmt.Errorf("unable to open %s: %v", fname, err)
			os.Exit(1)
		}
		_, err = fmt.Fprintf(f, "%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			results.GOMAXPROCS, min, q1, median, q3, max, rawMin, rawMax, gAvg, tot)
		if err != nil {
			fmt.Errorf("unable to write to %s: %v", fname, err)
			os.Exit(1)
		}
		if err = f.Close(); err != nil {
			fmt.Errorf("unable to close %s: %v", fname, err)
			os.Exit(1)
		}
	}
}

/******************************************************************************
 * Run qbench.                                                                *
 ******************************************************************************/

func bench(quit, dead chan struct{}) {
	// Prime our virtual memory space.
	benchChan(qbench.Cfg{
		Enqueuers: 100,
		Dequeuers: 100,
		Messages:  *messages,
	})

	// Leave GC on for benchmarks, imitating more standard program behavior.
	for _, enqueuers := range []int{100, 10, 1} {
		for _, dequeuers := range []int{100, 10, 1} {
			for _, cpu := range []int{1, 8, 16, 24, 32, 40, 48} {
				select {
				case <-quit:
					fmt.Println("Quitting.")
					close(dead)
					return
				default:
				}
				runtime.GOMAXPROCS(cpu)
				fmt.Printf("Bench on: %dproc, %denq, %ddeq\n", cpu, enqueuers, dequeuers)
				cfg := qbench.Cfg{
					Enqueuers: enqueuers,
					Dequeuers: dequeuers,
					Messages:  *messages,
				}
				fmt.Println("channel... ")
				results := benchChan(cfg)
				processResults("channel", results)
				runtime.GC()
				fmt.Println("mpmcdvq... ")
				results = benchMpMcDVq(cfg)
				processResults("mpmcdvq", results)
				runtime.GC()
				if enqueuers == 1 {
					fmt.Println("spmcdvq... ")
					results = benchSpMcDVq(cfg)
					processResults("spmcdvq", results)
					runtime.GC()
				}
				if dequeuers == 1 {
					fmt.Println("mpscdvq... ")
					results = benchMpScDVq(cfg)
					processResults("mpscdvq", results)
					runtime.GC()
				}
				if enqueuers == 1 && dequeuers == 1 {
					fmt.Println("spscdvq... ")
					results = benchSpScDVq(cfg)
					processResults("spscdvq", results)
					runtime.GC()
				}
				fmt.Println("done.")
			}
		}
	}
	close(dead)
}

func main() {
	flag.Parse()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGHUP)
	quit := make(chan struct{})
	dead := make(chan struct{})

	fmt.Println("Starting benchmarks...")
	go bench(quit, dead)
	select {
	case <-stop:
		fmt.Println("\nStop intercepted, waiting for current benchmark to finish.\n")
		close(quit)
		<-dead
	case <-dead:
		fmt.Println("Benchmarks finished.")
	}
}
