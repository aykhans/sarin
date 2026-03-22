# Benchmark

Compares [sarin](https://github.com/aykhans/sarin), [wrk](https://github.com/wg/wrk), and [bombardier](https://github.com/codesenberg/bombardier) against a minimal C HTTP server using epoll.

## Requirements

- `sarin`, `wrk`, `bombardier` in PATH
- `gcc`

## Usage

```bash
./benchmark/run.sh
```

Configuration is at the top of `run.sh`:

```bash
DURATION="30s"
CONNECTIONS=(50 100 200)
ITERATIONS=3
```

## Structure

```
benchmark/
  run.sh          - benchmark script
  server/
    server.c      - C epoll HTTP server (returns "ok")
  results/        - output directory (auto-created)
```

## Output

Each run produces per-tool files:

- `*.out` - tool stdout (throughput, latency)
- `*.time` - `/usr/bin/time -v` output (peak memory, CPU time)
- `*_resources.csv` - sampled CPU/memory during run

A summary table is printed at the end with requests/sec, total requests, elapsed time, and peak memory.
