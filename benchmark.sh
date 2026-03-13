#!/usr/bin/env bash

set -euo pipefail

RUNS=20
CMD="go run ./cmd/cli -U http://localhost:80 -r 1_000_000 -c 100"

declare -a times_default
declare -a times_gogcoff

echo "===== Benchmark: default GC ====="
for i in $(seq 1 $RUNS); do
    echo "Run $i/$RUNS ..."
    start=$(date +%s%N)
    $CMD
    end=$(date +%s%N)
    elapsed=$(( (end - start) / 1000000 )) # milliseconds
    times_default+=("$elapsed")
    echo "  -> ${elapsed} ms"
done

echo ""
echo "===== Benchmark: GOGC=off ====="
for i in $(seq 1 $RUNS); do
    echo "Run $i/$RUNS ..."
    start=$(date +%s%N)
    GOGC=off $CMD
    end=$(date +%s%N)
    elapsed=$(( (end - start) / 1000000 ))
    times_gogcoff+=("$elapsed")
    echo "  -> ${elapsed} ms"
done

echo ""
echo "============================================"
echo "                 RESULTS"
echo "============================================"

echo ""
echo "--- Default GC ---"
sum=0
for i in $(seq 0 $((RUNS - 1))); do
    echo "  Run $((i + 1)): ${times_default[$i]} ms"
    sum=$((sum + times_default[$i]))
done
avg_default=$((sum / RUNS))
echo "  Average: ${avg_default} ms"

echo ""
echo "--- GOGC=off ---"
sum=0
for i in $(seq 0 $((RUNS - 1))); do
    echo "  Run $((i + 1)): ${times_gogcoff[$i]} ms"
    sum=$((sum + times_gogcoff[$i]))
done
avg_gogcoff=$((sum / RUNS))
echo "  Average: ${avg_gogcoff} ms"

echo ""
echo "--- Comparison ---"
if [ "$avg_default" -gt 0 ]; then
    diff=$((avg_default - avg_gogcoff))
    echo "  Difference: ${diff} ms (positive = GOGC=off is faster)"
fi
echo "============================================"
