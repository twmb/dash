dash
====

Dash (for when you want to go fast) is a collection of Go packages implementing
fast algorithms. The code heavily references or directly transliterates other
projects, and is commented appropriately.

As the go runtime is unpredictable, these packages can only attempt to boost
performance over idiomatic Go. The packages are commented with their pitfalls
and benefits. Libraries internally heavily rely on unsafe or Go assembly,
sometimes forcing callers to use unsafe for max performance.

Experimental code is under `experimental/`. This code is theoretically good, as
they are direct transliterations of fast code in other languages, but has bad
performance in Go. The performance degradation most likely stems to an
uncontrollable runtime or unnecessary heap allocations.
