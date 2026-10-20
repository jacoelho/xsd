# Benchmarks

See the [benchmark results and measurement dates](README.md#benchmarks) in the
README.

Each result is the upper median (sixth sorted value) of 10 runs after one
warm-up per workload and validator. Time and peak RSS are summarized
independently.

In the 2026-10-20 Go run, elapsed times ranged from 24.315–25.739 s for
1 GiB and 48.620–49.584 s for 2 GiB. The 100,000-row identity workload ranged
from 300.193–329.040 ms. Streaming peak RSS upper medians stayed between
6.594 MiB and 6.938 MiB; the identity upper median was 89.219 MiB.

Libxml2 was not rerun. Its 2026-09-06 timings varied on the larger files:
20.86–37.68 s for 1 GiB and 51.08–52.20 s for 2 GiB across the 10 samples.
The Go and libxml2 columns therefore come from separate measurement dates,
rather than a paired comparison.

Timings include process startup, schema compilation, and validation. Go streams
input; libxml2 runs in tree mode with `--huge --noout --schema`. Peak RSS is
process memory reported by `/usr/bin/time`, not cumulative Go allocations.
These synthetic workloads are not a production traffic mix or a comparison of
libxml2's streaming mode.

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
  "$libxml2" --huge --noout --schema "$schema" "$xml" # warm-up
  for sample in 1 2 3 4 5 6 7 8 9 10; do
    /usr/bin/time -l "$libxml2" --huge --noout --schema "$schema" "$xml"
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
