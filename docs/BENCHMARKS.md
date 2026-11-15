# Benchmarks

See the [benchmark results and measurement dates](../README.md#benchmarks) in the
README.

Each result is the upper median (sixth sorted value) of 10 runs after one
warm-up per workload and validator. CPU time, elapsed time, and peak RSS are
summarized independently.

The [benchmark harness](../tests/large_benchmark_test.go) runs both validators
itself, alternating them run by run so a drift in machine load affects both
alike. CPU time is the user plus system time and peak RSS is the maximum
resident set size that the kernel accounts to each child process (`wait4`
rusage, microsecond resolution); no wrapper process is timed. CPU time and
elapsed time include process startup, schema compilation, and validation. Go
streams input; libxml2 uses `--stream --huge --noout --schema`. These synthetic
workloads are not a production traffic mix.

The harness generates documents from 20 MiB through 2 GiB, plus 100,000 rows
with ID/IDREF, key, unique, and keyref constraints. It runs the repository's
`bin/xmllint` with input-byte and identity-entry limits that admit each
generated document.

To reproduce the README table, run:

```sh
make bench-libxml2
```

It builds `bin/xmllint`, prints the libxml2 version, and runs the harness with
`XSD_LARGE_RUNS=10` and `XSD_LARGE_LIBXML2=/usr/bin/xmllint`. Set `LIBXML2` to
select another libxml2 executable explicitly, so it cannot be confused with the
repository's Go binary, and `LARGE_RUNS` to change the sample count. The
summary prints, per workload, each validator's CPU time, elapsed time, and peak
RSS, and the Go-to-libxml2 CPU and RSS ratios.

Without `XSD_LARGE_LIBXML2` the harness measures `bin/xmllint` alone. For a
smoke run, replace `XSD_LARGE_RUNS=10` with
`XSD_LARGE_SIZE_BYTES=1048576 XSD_LARGE_IDENTITY_ROWS=1000 XSD_LARGE_RUNS=1`.
The harness removes generated files unless `XSD_LARGE_DIR` names a directory
to keep them in; remove that directory when finished.

## Session reuse

For persistent workers with allocation-heavy overlapping bursts, measure one
session per worker, reused sequentially. It avoids repeatedly discarding scratch
when calls compete for the engine's single idle slot. Each session retains its
own bounded scratch; bound the worker count and compare retained heap, throughput,
and allocations under both steady traffic and overlapping bursts. The
`BenchmarkSessionOwnership` benchmarks cover these workloads.

## Commit comparisons

These in-process benchmarks exclude process startup and compilation, so two
commits can be compared on the same harness:

| Workload | Benchmark |
| --- | --- |
| 20 MiB streaming validation | `BenchmarkLargeStreamingValidation` (`tests`) |
| 100,000-row identity validation | `BenchmarkLargeIdentityValidation` (`tests`) |
| Peak in-document heap, identity | `BenchmarkLargeIdentityPeakHeap` (`tests`) |
| Retained compiled engine | `BenchmarkCompileRetainedContentModels` (root) |

`BenchmarkLargeIdentityPeakHeap` reports `peak-HeapInuse-delta-B` and
`peak-HeapInuse-B` from a 1 ms `HeapInuse` sampler with GC percent pinned to 100.
`BenchmarkCompileRetainedContentModels` reports `retained-B/op`, the live-heap
delta across `Compile` after two collections on each side. Each metric is the
median over iterations. Both benchmarks call `runtime.ReadMemStats` or
`runtime.GC` inside the loop, so their `ns/op` is not a timing. The sampler is a
lower bound on the true peak; run it at `-benchtime=1x` with several `-count`
samples and `-cpu` of at least 2, so the sampler is not starved by validation.

Function placement in the linked binary can move a benchmark by 1–2% when
only unrelated packages change. Before attributing such a delta to a commit,
build both commits with several `-ldflags=-randlayout=N` seeds and compare the
per-seed medians; a delta that vanishes across seeds is a layout artifact.
Run the base and candidate binaries alternately, then repeat with their order
swapped: a delta that follows the binary is real, and one that follows the run
position is an ordering effect.
