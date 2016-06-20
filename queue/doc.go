// Package queue contains implementations of queues.
//
// Multi-producer, multi-consumer queues are located in mpmc/, with their
// single analogues located in spmc/, mpsc/, or spsc/.
//
// Queue's take unsafe.Pointer's to enqueue, and return those same pointers on
// dequeue. This is done to eliminate the need of a heap allocated interface
// that contains a pointer to the heap allocated variable you are enqueueing.
//
// {m,s}p{m,s}cdvq's contains a transliteration of Dmitry Vyukov's mpmc bounded queue,
// www.1024cores.net/home/lock-free-algorithms/queues/bounded-mpmc-queue.
package queue
