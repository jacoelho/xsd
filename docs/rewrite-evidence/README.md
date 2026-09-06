# Rewrite evidence packet

This directory contains portable inputs and the final R9 evidence for the
streaming rewrite. The assessment, including small timing and allocation
trade-offs, is in [../rewrite-performance.md](../rewrite-performance.md).

The final measurements use:

- baseline: `2764e554dc67e3632e1bf3880e1dc2bfd0ef4cb3`
- production revision: `34142ca15e319c9f3e206b947789dcd890c8bb80`
- toolchain: Go 1.27.0 on `darwin/arm64`, Apple M2 Max
- six alternating 200 ms samples per benchmark and revision

[performance-r9](performance-r9/summary.md) contains normalized samples,
metadata, raw aggregate outputs, and statistical comparisons.
[verification-r9.json](verification-r9.json) records the passing repository
gates and final retention observations. The source worktree was clean during
measurement; the baseline contained only the recorded benchmark fixtures.
`provenance.json` retains earlier observations under their historical revision
and adds the final packet hashes. Historical files do not describe final timings.

## Contents

| Path | Purpose |
| --- | --- |
| `benchmarks/runner.py` | Builds test binaries and runs an alternating baseline/current matrix. |
| `benchmarks/manifest.json` | Benchmark groups, package paths, CPU settings, and benchmark expressions. |
| `benchmarks/baseline-benchmarks.patch` | Benchmark-only fixtures required by the baseline worktree. |
| `benchmarks/export-performance-r5.py` | Validates and exports an existing paired sample directory; it never runs a benchmark or declares parity. |
| `value-probe/generate-baseline.go.txt` | Generator for the historical baseline API (`internal/compile` and `internal/runtime`). |
| `value-probe/generate-current.go.txt` | Generator for the current API (`internal/value`). Both sources remain `.go.txt` so this directory is not a Go package. |
| `value-probe/compare.py` | Compares acceptance, unsupported status, and identity equivalence partitions. |
| `value-probe/cases.json` | 374 probe strings used for each of 45 builtin types. |
| `retention/probe.go.txt` | Flat-stream heap-retention probe; it is also kept out of the Go package graph. |
| `verification-r9.json` | Final full-gate, corpus, and retention metadata. |
| `performance-r9/` | Final raw/normalized samples, metadata, and comparisons. |
| `verification/r9-gates-final.txt` | Complete passing repository gate log. |
| `retention/*-r9*` | Final sampled heap and whole-process RSS observations. |
| `verification-r5.json` | Historical full-gate and corpus metadata. |
| `value-probe/historical-*.{txt,json}` | Small historical comparison summaries and the one recorded acceptance delta. |
| `retention/historical-*.jsonl` | Three small historical retention result rows per revision. |

Test binaries, profiles, per-round raw files, and the 16,830-row value outputs
remain in the ignored `.lab/rewrite/` workspace. The final packet includes raw
aggregate benchmark outputs and normalized per-round samples; metadata records
the individual input and binary hashes. Historical value output hashes and
counts remain in `provenance.json`.

## Paired benchmark matrix

Create detached worktrees and apply the fixture patch only to the baseline:

```sh
git worktree add --detach /tmp/xsd-baseline 2764e554dc67e3632e1bf3880e1dc2bfd0ef4cb3
git worktree add --detach /tmp/xsd-current 34142ca15e319c9f3e206b947789dcd890c8bb80
git -C /tmp/xsd-baseline apply --unidiff-zero "$PWD/docs/rewrite-evidence/benchmarks/baseline-benchmarks.patch"
```

Run the matrix from this checkout. The output directory must not already exist;
it contains test binaries and raw samples and should stay ignored:

```sh
python3 docs/rewrite-evidence/benchmarks/runner.py \
  --baseline /tmp/xsd-baseline \
  --current /tmp/xsd-current \
  --output /tmp/xsd-r9-matrix \
  --rounds 6 \
  --benchtime 200ms
```

Use repeated `--group` options to run a subset. Each output records source
revisions, worktree status, Go version, binary SHA-256 values, alternating
execution order, CPU settings, and raw benchmark rows. The runner compares
neither timings nor acceptance; review those rows separately.

The baseline patch changes benchmark fixtures and test-only exports. It does
not change the baseline production implementation and must not be applied to
the current worktree.

The portable runner writes `<group>-baseline.txt` and `<group>-current.txt`;
compare them with `benchstat` (use `-ignore pkg` for moved packages).
Its metadata and samples reproduce the workloads independently of the local
capture layout. To normalize the original prefixed capture directory, use the
exporter. It
accepts any numeric prefix, so R9 samples use `--prefix r9` without
editing the script:

```sh
python3 docs/rewrite-evidence/benchmarks/export-performance-r5.py \
  --input-dir "$PWD/.lab/rewrite/final-perf" \
  --output-dir "$PWD/.lab/rewrite/performance-export-r9" \
  --prefix r9 \
  --status draft
```

The exporter requires six successful 200 ms rounds per listed group, matching
rows after its documented validation-name normalization. It preserves input
hashes and source metadata, but does not calculate a performance acceptance
ratio. `--status historical` is the default; `draft` is available for a new
review packet and still does not assert parity.

## Value differential

Each generator must run inside its module tree because it imports an internal
package. Copy the role-specific source into each ignored worktree before running:

```sh
for role in baseline current; do
  root=/tmp/xsd-$role
  mkdir -p "$root/.lab/rewrite/value-probe"
  source=docs/rewrite-evidence/value-probe/generate-$role.go.txt
  cp "$source" \
    "$root/.lab/rewrite/value-probe/generate.go"
  cp docs/rewrite-evidence/value-probe/cases.json \
    "$root/.lab/rewrite/value-probe/cases.json"
  (cd "$root" && go run .lab/rewrite/value-probe/generate.go \
    .lab/rewrite/value-probe/cases.json \
    > "/tmp/xsd-$role-value.jsonl")
done
python3 docs/rewrite-evidence/value-probe/compare.py \
  --baseline /tmp/xsd-baseline-value.jsonl \
  --rewrite /tmp/xsd-current-value.jsonl
```

The resolver in the probe admits only the synthetic `item` QName and notation
binding. The comparison requires identical keys, allows exactly the recorded
large-duration acceptance change, rejects unsupported-status changes, and
compares identity equivalence partitions for jointly accepted values. It does
not require identity byte strings to remain identical across implementations.

The historical run produced 16,830 rows, 3,103 jointly accepted values, equal
identity partitions, no unsupported-status changes, and one intended acceptance
delta: `duration:P999999999999999999999999999Y`. The raw outputs are omitted
from this packet because each is about 1.5 MiB; their hashes are in
`provenance.json`.

## Flat-stream retention

Copy the retention source into an ignored directory in each worktree before
running it:

```sh
for role in baseline current; do
  root=/tmp/xsd-$role
  mkdir -p "$root/.lab/rewrite"
  cp docs/rewrite-evidence/retention/probe.go.txt \
    "$root/.lab/rewrite/retention-probe.go"
  (cd "$root" && GOMAXPROCS=1 go run .lab/rewrite/retention-probe.go \
    -label="$role" -out=".lab/rewrite/retention-$role.jsonl")
done
```

The probe generates a flat XML stream at 1, 16, and 128 MiB, validates it
through the public API, samples heap statistics every 256 reads, and reports
post-GC heap allocation and total allocation. It does not retain the generated
document. The final current snapshot plateaued at 188,104 post-GC heap bytes for
16 and 128 MiB, compared with 238,872 for the baseline; warmed total allocation
was 656 bytes at those sizes. The final RSS observations used compiled probe
binaries under `/usr/bin/time -l`, one process per revision covering all three
sizes. RSS therefore includes process startup and is not a per-size heap
measurement. The report gives the exact values and scope.

The Go sources intentionally use `.go.txt`. A permanent `.go` file here would
be discovered by `go test ./...` and would add an evidence helper as a public
package. Copying into `.lab/rewrite/` keeps the helper inside the module while
preserving the repository package graph.
