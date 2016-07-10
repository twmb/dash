// Package qbench benchmarks concurrent enqueueing and dequeueing sized queues.
package qbench

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/twmb/dash/bench/etime"
)

var nowOverhead int64

func init() {
	iters := int64(100000000)
	start := etime.Now()
	for i := int64(0); i < iters; i++ {
		_ = etime.Now()
	}
	end := etime.Now()
	nowOverhead = (end - start) / iters
}

// Interface is used to enqueue and dequeue in benchmarks.
type Interface interface {
	Enqueue(unsafe.Pointer)
	Dequeue() unsafe.Pointer
}

// Cfg is the configuration used to run a benchmark. As queues in the dash repo
// are forced to multiplier-of-2 sizes, _all_ benchmarks against dash queues
// should also use multiplier-of-2 sizes.
type Cfg struct {
	// Enqueuers is the count of enqueuers to use.
	Enqueuers int
	// Dequeuers is the count of dequeuers to use.
	Dequeuers int
	// Messages is the count of messages to send through Impl. qbench
	// pre-over-sizes timing slices so as to not reallocate in the middle
	// of benchmarking, meaning each benchmark will have uses quite a bit
	// of memory. Users must ensure their RAM can support this.
	Messages int
	// Impl is the queue.
	Impl Interface
}

// Results contains the results of one Interface benchmark for a given Cfg.
type Results struct {
	// GOMAXPROCS is the GOMAXPROCS setting for this benchmark.
	GOMAXPROCS int
	// Enqueuers is how many enqueuers used.
	Enqueuers int
	// Dequeuers is how many dequeuers used.
	Dequeuers int
	// EnqueueTimings contains etime deltas for all enqueues.
	EnqueueTimings [][]int64
	// DequeueTimings contains etime deltas for all dequeues.
	DequeueTimings [][]int64
	// ThroughputTimings contains etime deltas for the entire timing of
	// sending a message through a queue, meaning just before enqueue to
	// just after dequeue.
	ThroughputTimings [][]int64
	// TotalTiming captures the etime delta from immediately before allowing
	// all enqueue and dequeue goroutines to start and immediately after
	// all end.
	TotalTiming int64
}

// benchEnqueuer runs enqueueing to our queue interface <enqueues> times,
// tracking the runtime of each enqueue in timings.
type benchEnqueuer struct {
	enqImpl    Interface
	enqTimings []int64
	enqueues   int
}

func (bq *benchEnqueuer) run(begin chan struct{}, wg *sync.WaitGroup) {
	<-begin
	for i := 0; i < bq.enqueues; i++ {
		start := etime.Now()
		bq.enqImpl.Enqueue(unsafe.Pointer(&start))
		end := etime.Now()
		bq.enqTimings = append(bq.enqTimings, end-start-nowOverhead)
	}
	wg.Done() // defer is currently slow; avoid the overhead in timings
}

// benchDequeuer runs dequeueing from our queue interface <enqueues> times,
// tracking the runtime of each enqueue in timings.
type benchDequeuer struct {
	deqImpl    Interface
	dequeues   int
	timings    []int64
	deqTimings []int64
}

func (bq *benchDequeuer) run(begin chan struct{}, wg *sync.WaitGroup) {
	<-begin
	for i := 0; i < bq.dequeues; i++ {
		start := etime.Now()
		enqStart := *(*int64)(bq.deqImpl.Dequeue())
		end := etime.Now()
		bq.timings = append(bq.timings, end-enqStart-nowOverhead)
		bq.deqTimings = append(bq.deqTimings, end-start-nowOverhead)
	}
	wg.Done()
}

// Bench runs concurrent enqueuers and dequeuers based off the given config,
// returning the timing results on completion.
func Bench(cfg Cfg) Results {
	// Synchronization variables for benchEnqueuer and benchDequeuer.
	begin := make(chan struct{})
	var wg sync.WaitGroup

	// Begin all enqueuers.
	enqDiv, enqRem := cfg.Messages/cfg.Enqueuers, cfg.Messages%cfg.Enqueuers
	enqTimings := make([]*[]int64, 0, cfg.Enqueuers)
	for i := 0; i < cfg.Enqueuers; i++ {
		enqueues := enqDiv
		if enqRem > 0 {
			enqueues++
			enqRem--
		}
		bencher := &benchEnqueuer{
			enqImpl:    cfg.Impl,
			enqueues:   enqueues,
			enqTimings: make([]int64, 0, cfg.Messages),
		}
		enqTimings = append(enqTimings, &bencher.enqTimings)
		wg.Add(1)
		go bencher.run(begin, &wg)
	}

	// Begin all dequeuers.
	deqDiv, deqRem := cfg.Messages/cfg.Dequeuers, cfg.Messages%cfg.Dequeuers
	timings := make([]*[]int64, 0, cfg.Dequeuers)
	deqTimings := make([]*[]int64, 0, cfg.Dequeuers)
	for i := 0; i < cfg.Dequeuers; i++ {
		dequeues := deqDiv
		if deqRem > 0 {
			dequeues++
			deqRem--
		}
		bencher := &benchDequeuer{
			deqImpl:    cfg.Impl,
			dequeues:   dequeues,
			timings:    make([]int64, 0, cfg.Messages),
			deqTimings: make([]int64, 0, cfg.Messages),
		}
		timings = append(timings, &bencher.timings)
		deqTimings = append(deqTimings, &bencher.deqTimings)
		wg.Add(1)
		go bencher.run(begin, &wg)
	}

	start := etime.Now()
	// Start all enqueuers and dequeuers.
	close(begin)
	// Wait for all to finish.
	wg.Wait()
	end := etime.Now()
	total := end - start - nowOverhead

	b := Results{
		GOMAXPROCS:        runtime.GOMAXPROCS(0),
		Enqueuers:         cfg.Enqueuers,
		Dequeuers:         cfg.Dequeuers,
		EnqueueTimings:    make([][]int64, 0, len(enqTimings)),
		DequeueTimings:    make([][]int64, 0, len(deqTimings)),
		ThroughputTimings: make([][]int64, 0, len(timings)),
		TotalTiming:       total,
	}

	for _, timingPtr := range enqTimings {
		timing := *timingPtr
		b.EnqueueTimings = append(b.EnqueueTimings, timing)
	}
	for _, timingPtr := range deqTimings {
		timing := *timingPtr
		b.DequeueTimings = append(b.DequeueTimings, timing)
	}
	for _, timingPtr := range timings {
		timing := *timingPtr
		b.ThroughputTimings = append(b.ThroughputTimings, timing)
	}
	return b
}
