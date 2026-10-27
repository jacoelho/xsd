# Benchmarks

See the [benchmark results and measurement dates](README.md#benchmarks) in the
README.

Each result is the upper median (sixth sorted value) of 10 runs after one
warm-up per workload and validator. Time and peak RSS are summarized
independently.

In the 2026-10-26 Go run, displayed elapsed times ranged from 21.741–22.505 s
for 1 GiB and 42.987–46.535 s for 2 GiB. The 100,000-row identity workload
ranged from 329.757–340.116 ms. Streaming peak RSS upper medians stayed between
6.781 MiB and 6.938 MiB; the identity upper median was 89.047 MiB.
The 20 MiB samples ranged from 448.366–800.711 ms; all ten samples were retained.

Libxml2 was rerun with `--stream` on the same files after the Go suite. Its
elapsed times ranged from 16.92–17.67 s for 1 GiB, 33.42–35.41 s for 2 GiB,
and 0.42–0.69 s for identity constraints. Streaming peak RSS upper medians were
1.703–1.734 MiB; the identity upper median was 74.344 MiB. These sequential
cohorts are not paired measurements. Earlier tree-mode figures use a different
execution mode and should not be compared as evidence of a code speedup.

Timings include process startup, schema compilation, and validation. Go streams
input; libxml2 uses `--stream --huge --noout --schema`. The Go harness measures
elapsed time around the timed child process; libxml2 elapsed time is the
`/usr/bin/time -l` real value, at 0.01 s resolution. Peak RSS is process memory
reported by `/usr/bin/time`, not cumulative Go allocations. These synthetic
workloads are not a production traffic mix.

The [benchmark harness](tests/large_benchmark_test.go) generates documents from
20 MiB through 2 GiB, plus 100,000 rows with ID/IDREF, key, unique, and keyref
constraints. It runs the repository's `bin/xmllint` and sets input-byte and
identity-entry limits to admit each generated document.

To reproduce the Go measurements and retain the inputs for libxml2:

```sh
make xmllint
benchmark_dir=$(mktemp -d)
XSD_LARGE_BENCHMARK=1 XSD_LARGE_RUNS=10 XSD_LARGE_DIR="$benchmark_dir" \
  go test ./tests -run '^TestLargeXMLLintBenchmark$' -timeout=45m -count=1 -v
```

Then run libxml2 on those same inputs. Select its executable explicitly so it
cannot be confused with the repository's Go binary. This command uses macOS
`/usr/bin/time -l`; use `-v` on Linux. Discard the warm-up, then sort
the 10 elapsed times and 10 peak-RSS measurements separately and take the sixth
value from each to match the Go harness.

```sh
libxml2=/usr/bin/xmllint
"$libxml2" --version
for xml in "$benchmark_dir"/streaming/*/document.xml "$benchmark_dir"/identity/document.xml; do
  case "$xml" in
    */streaming/*) schema="$benchmark_dir/streaming/schema.xsd" ;;
    *) schema="$benchmark_dir/identity/schema.xsd" ;;
  esac
  "$libxml2" --stream --huge --noout --schema "$schema" "$xml" # warm-up
  for sample in 1 2 3 4 5 6 7 8 9 10; do
    /usr/bin/time -l "$libxml2" --stream --huge --noout --schema "$schema" "$xml"
  done
done
```

For a smoke run, add
`XSD_LARGE_SIZE_BYTES=1048576 XSD_LARGE_IDENTITY_ROWS=1000 XSD_LARGE_RUNS=1`
to the Go command. Without `XSD_LARGE_DIR`, the harness removes generated
files; with it, remove the retained directory when finished.

## Session reuse

For persistent workers with allocation-heavy overlapping bursts, measure one
session per worker, reused sequentially. It avoids repeatedly discarding scratch
when calls compete for the engine's single idle slot. Each session retains its
own bounded scratch; bound the worker count and compare retained heap, throughput,
and allocations under both steady traffic and overlapping bursts. The
`BenchmarkSessionOwnership` benchmarks cover these workloads.
