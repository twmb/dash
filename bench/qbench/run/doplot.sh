#!/bin/bash
for f in `ls plot* | xargs`
do
	gnuplot "$f"
done
