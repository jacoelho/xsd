# Rewrite evidence packet

This directory contains small, portable inputs for reproducing the rewrite
checks. Its recorded observations are historical. They do not establish final
parity for the current worktree or close the performance gate.

The historical snapshot used:

- baseline: `2764e554dc67e3632e1bf3880e1dc2bfd0ef4cb3`
- current verification tree: `0b5ea81af97973f6a05db363a51e18919d02f7c7`
- production revision named by the verification record:
  `544432c65a634198c4d551f2775970cef49d5406`
- toolchain: Go 1.27.0 on `darwin/arm64`, Apple M2 Max

See [`provenance.json`](provenance.json) for source hashes and the exact
historical counts. The current repository may have moved beyond those commits;
use detached worktrees at the recorded revisions when reproducing the snapshot.

## Contents

| Path | Purpose |
| --- | --- |
| `benchmarks/runner.py` | Builds test binaries and runs an alternating baseline/current matrix. |
| `benchmarks/manifest.json` | Benchmark groups, package paths, CPU settings, and benchmark expressions. |
| `benchmarks/baseline-benchmarks.patch` | Benchmark-only fixtures required by the historical baseline worktree. |
| `benchmarks/export-performance-r5.py` | Validates and exports an existing paired sample directory; it never runs a benchmark or declares parity. |
| `value-probe/generate-baseline.go.txt` | Generator for the historical baseline API (`internal/compile` and `internal/runtime`). |
| `value-probe/generate-current.go.txt` | Generator for the current API (`internal/value`). Both sources remain `.go.txt` so this directory is not a Go package. |
| `value-probe/compare.py` | Compares acceptance, unsupported status, and identity equivalence partitions. |
| `value-probe/cases.json` | 374 probe strings used for each of 45 builtin types. |
| `retention/probe.go.txt` | Flat-stream heap-retention probe; it is also kept out of the Go package graph. |
| `verification-r5.json` | Historical full-gate and corpus metadata. |
| `value-probe/historical-*.{txt,json}` | Small historical comparison summaries and the one recorded acceptance delta. |
| `retention/historical-*.jsonl` | Three small historical retention result rows per revision. |

Large benchmark outputs, binaries, profiles, and the 16,830-row value output
files remain in the ignored `.lab/rewrite/` workspace. Their hashes and row
counts are recorded in `provenance.json`.

## Paired benchmark matrix

Create detached worktrees and apply the fixture patch only to the baseline:

```sh
git worktree add --detach /tmp/xsd-baseline 2764e554dc67e3632e1bf3880e1dc2bfd0ef4cb3
git worktree add --detach /tmp/xsd-current 0b5ea81af97973f6a05db363a51e18919d02f7c7
git -C /tmp/xsd-baseline apply --unidiff-zero "$PWD/docs/rewrite-evidence/benchmarks/baseline-benchmarks.patch"
```

Run the matrix from this checkout. The output directory must not already exist;
it contains test binaries and raw samples and should stay ignored:

```sh
python3 docs/rewrite-evidence/benchmarks/runner.py \
  --baseline /tmp/xsd-baseline \
  --current /tmp/xsd-current \
  --output /tmp/xsd-r5-matrix \
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

To normalize an already completed sample directory, use the exporter. It
accepts any numeric prefix, so later R6 samples can use `--prefix r6` without
editing the script:

```sh
python3 docs/rewrite-evidence/benchmarks/export-performance-r5.py \
  --input-dir "$PWD/.lab/rewrite/final-perf" \
  --output-dir "$PWD/.lab/rewrite/performance-export-r5" \
  --prefix r5 \
  --status historical
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
document. The historical current snapshot plateaued at 188,288 post-GC heap
bytes for 16 and 128 MiB, compared with 238,944 for the baseline; warmed total
allocation was 800 bytes at those sizes. These are historical observations, not
a current-tree guarantee.

The Go sources intentionally use `.go.txt`. A permanent `.go` file here would
be discovered by `go test ./...` and would add an evidence helper as a public
package. Copying into `.lab/rewrite/` keeps the helper inside the module while
preserving the repository package graph.
