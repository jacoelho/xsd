# Rewrite performance and verification

The rewrite is complete for the supported feature scope at production revision
`34142ca15e319c9f3e206b947789dcd890c8bb80`, compared with baseline
`2764e554dc67e3632e1bf3880e1dc2bfd0ef4cb3`. The 76 public workloads have a
**25.82% lower geometric-mean time**. The full repository verification contract
passes. Feature scope, reference comparisons, and capability evidence are indexed
in [rewrite-assessment.md](rewrite-assessment.md) and
[reference-comparison.md](reference-comparison.md); [ARCHITECTURE.md](../ARCHITECTURE.md)
owns the implemented design.

This is an overall performance assessment, not a claim that every metric
improves. Small internal timing regressions and several compilation allocation
trade-offs remain explicit below. Schema-text compilation was slower in the
main matrix; a longer confirmation run put the difference within noise.

## Method and evidence

R9 used Go 1.27.0 on Darwin/arm64, Apple M2 Max, six alternating baseline/current
rounds, and 200 ms per benchmark. Each CPU setting ran in a separate process:
one CPU normally, and one/eight CPUs for concurrent validation. All 11 groups
record a clean current worktree. The baseline production source is unchanged;
its recorded changes supply equivalent benchmark fixtures and test-only exports.

The main matrix contains 1,812 samples and 151 distinct paired row identities.
One internal union row compares different return contracts and is diagnostic;
the remaining 150 are matching comparisons. Baseline identity `/direct_match`
and `/direct_miss` names map to current `/compiled_match` and `/compiled_miss`.
Those normalized rows are included; only the retired generic adapter rows are
dropped. The matching public late-union benchmark is also included.

[The evidence packet](rewrite-evidence/README.md) preserves raw aggregate
outputs, normalized per-round CSV samples, binary and input hashes, source
status, toolchain, statistical comparisons, and reproduction inputs. The public
summary reports benchstat's geometric means, including its zero-value caveats
for allocation summaries. Supporting group timing figures use ratios of
workload medians. Neither weights workloads by an assumed production traffic
mix. `paired-geomeans.json` supplies a separate median-ratio cross-check; its
allocation calculation excludes pairs with either value zero and can therefore
differ from the public benchstat summary.

The host also ran an unrelated benchmark capture process, recorded in the
metadata. Alternating order reduces drift, but this was not an isolated-machine
experiment. Small differences should not be read as precise universal bounds.
Benchstat significance describes these samples; it does not prove equivalence
for unmeasured workloads or a different machine.

## Public results

| Metric | Baseline | Rewrite | Change |
| --- | ---: | ---: | ---: |
| Time, geometric mean over 76 workloads | 352.7 µs | 261.6 µs | -25.82% |
| Throughput, workloads reporting bytes processed | 18.15 MiB/s | 23.57 MiB/s | +29.87% |
| Allocated bytes, geometric mean over nonzero pairs | — | — | -27.11% |
| Allocations, geometric mean over nonzero pairs | — | — | -15.19% |

Representative complete public workloads:

| Workload | Time change |
| --- | ---: |
| Small-schema compilation | -47.46% |
| Compilation and first session | -47.12% |
| Repeated QName values | -56.60% |
| `all` content with 1,024 members | -88.23% |
| Deep simple-type chain compilation | -50.74% |
| Distinct source-document compilation | -57.64% |
| 16 MiB streamed schema compilation | -61.01% |
| Duplicate-attribute validation | Within noise, p=0.818 |

Schema-text compilation was the only significantly slower public timing row
in the main matrix: 317.3 µs versus 334.1 µs, **+5.29%**, p=0.009. Six additional
alternating **1 s** samples with the same binaries measured 319.4 µs versus
325.8 µs, about **+2.0%**, p=0.132. Earlier R6–R8 runs also placed that workload
within noise. The evidence therefore does not establish a persistent material
timing regression, but the slower R9 result is retained, not discarded. Its
additional allocated bytes are stable and recorded below.

## Supporting workloads and trade-offs

| Group | Matching rows | Time geometric-mean change |
| --- | ---: | ---: |
| Formatting | 4 | -26.05% |
| XSD regular expressions | 10 | -42.27% |
| Typed values, excluding the diagnostic raw-union API | 5 | -14.92% |
| XML tokenization | 7 | -2.17% |
| Namespace admission | 12 | -17.75% |
| Duration parsing | 2 | -28.80% |
| Validation helpers, with normalized identity names | 14 | -1.76% |
| Concurrent public validation | 2 | -1.46% |
| XML whitespace | 6 | +1.01%, individual rows within noise |
| URI references | 12 | -0.16%, individual rows within noise |

Standalone well-formedness checking is faster or within noise at depths 10,
100, and 1,000, using 17/23/29 allocations versus 24/30/36. Formatting improves
across all four workloads and reduces allocated bytes by 54.66% overall.

Two narrow internal costs remain visible: CDATA crossing a reader-buffer
boundary is **3.80% slower** (572.4 versus 594.1 µs), and duplicate schema-location
hint recording is **5.93%/4.33% slower** at one/256 namespaces (about 16/12 ns).
The latter remains allocation-free. These results limit any claim of uniform
speedup; they do not outweigh the complete public workload improvements.

Compilation also trades allocation shape against execution time:

| Workload | Allocated bytes | Allocations | Time |
| --- | ---: | ---: | ---: |
| Deep simple-type chain | +4.47% | 8,478 → 12,899, +52.15% | -50.74% |
| Repeated nested union members | -2.34% | 4,784 → 5,228, +9.28% | Within noise |
| Regex category compilation | +24.29% | -44.56% | -10.91% |
| Substitution-group compilation | +18.93% | -4.57% | Within noise |
| Schema-text compilation | +20.98% | 360 → 282, -21.67% | See confirmation above |

Streamed schema compilation stays at approximately **122.8 KiB and 275
allocations** for 4 KiB, 1 MiB, and 16 MiB inputs. The baseline rises from
162.9 KiB to 35,528.6 KiB. This workload exercises discarded source payload;
retained semantic declarations remain proportional to admitted schema content.

The internal `PublishedRawUnionLateMember` row is diagnostic because the old
API returns `bool+error` and the new one returns a typed `Value+error`. Its
63.50% increase is not an equivalent-work acceptance ratio. The complete public
`SessionValidateUnionLateMember` workload performs matching validation and is
6.25% faster.

## Stream retention

The final retention probe generates input in 4,096-byte chunks, warms a public
session, samples heap statistics every 256 reads, and measures post-GC heap.
Neither input document is materialized by the probe.

| Input | Baseline post-GC heap | Rewrite post-GC heap | Baseline sampled peak heap | Rewrite sampled peak heap | Rewrite total allocation after warm-up |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1 MiB | 232,144 B | 181,376 B | 232,560 B | 181,728 B | 672 B |
| 16 MiB | 238,872 B | 188,104 B | 239,288 B | 188,456 B | 656 B |
| 128 MiB | 238,872 B | 188,104 B | 239,288 B | 188,456 B | 656 B |

Baseline warmed total allocation was 816/800/800 B. Both final heap measures
plateau at 16 and 128 MiB. `/usr/bin/time -l` recorded maximum resident sets of
8,257,536 B for baseline and 8,142,848 B for the rewrite; peak memory footprints
were 4,506,104 B and 4,473,360 B. Each RSS observation covers one compiled probe
process running all three sizes, including startup. It is not a separate RSS
measurement for each size or a statistically established RSS improvement.

These measurements establish the tested flat-stream behavior. Samples can miss
an instantaneous peak; they do not prove constant memory for arbitrary depth,
large scalar values, or identity tuples. Those dimensions have explicit limits
and owning adversarial tests documented in the architecture and assessment.

## Feature and verification result

All required targets pass on the final production tree: `make test`, `make race`,
`make wasm-test`, `make web-test`, `make browser-test`, `make fuzz-smoke`,
`make bench-smoke`, `make staticcheck`, and `make lint`; formatting and
`git diff --check` also pass. The full log is included in the evidence packet.

The corpus contains 14,548 manifest cases, 14,498 schema cases, and 25,199
instance runs. The unsupported inventory falls from 799 to 174, with 625 removed
and none added. The removed regex cases were individually adjudicated: 624 valid
schemas and one intentional `schema.facet` failure, plus 937 dependent instance
runs without mismatches. The targeted 16,830-pair datatype differential found
one intended huge-duration acceptance improvement, no unsupported changes, and
matching identity equivalence partitions for all 3,103 jointly accepted values.
That differential remains evidence for its enumerated inputs, not every value.

XML 1.0, UTF-8, explicit schema sources, synchronous stream processing, and no
library-owned network loading remain binding. Complete XSD 1.0 coverage is not
claimed: the 174-entry allowlist, including `xs:redefine`, remains explicit.
