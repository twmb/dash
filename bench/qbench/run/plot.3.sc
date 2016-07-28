set terminal png size 1280,768 font "helvetica neue thin,12"

set boxwidth 0.30 absolute
set offset 0.5,0.5,0,0

set xrange [ 0 : 49 ]
set ylabel 'nanoseconds'
set xlabel 'cpu'
set logscale y

set key left

enqs = 100
deqs = 1

types  = "enq deq thr"
typesFull = "enqueue dequeue throughput"

do for [i=1:words(types)] {
    timings = word(types, i)
    set output sprintf('e%dd%d.%s.png', enqs, deqs, timings)
    set title sprintf('%d enq, %d deq, %s timings', enqs, deqs, word(typesFull, i))

    plot sprintf('e%dd%d.%s.channel', enqs, deqs, timings) using 1:1:xticlabels(1) lt -3 notitle,\
           '' using ($1-0.60):3:2:6:5 with candlesticks lt 1 lw 3 title 'channel' whiskerbars,\
           '' using ($1-0.60):4:4:4:4 with candlesticks lt -1 lw 3 notitle,\
         sprintf('e%dd%d.%s.mpmcdvq', enqs, deqs, timings) using ($1-0.15):3:2:6:5 with candlesticks lt 3 lw 3 title 'mpmcdvq' whiskerbars,\
           '' using ($1-0.15):4:4:4:4 with candlesticks lt -1 lw 3 notitle, \
         sprintf('e%dd%d.%s.mpscdvq', enqs, deqs, timings) using ($1+0.35):3:2:6:5 with candlesticks lt 7 lw 3 title 'mpscdvq' whiskerbars,\
           '' using ($1+0.35):4:4:4:4 with candlesticks lt -1 lw 3 notitle
}
