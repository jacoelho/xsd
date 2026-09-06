# Final performance samples (R9)

Status: assessed. [The final report](../../rewrite-performance.md) evaluates
these samples, the longer schema-text confirmation, and retention evidence.
The exporter itself only validates and normalizes samples.

The export contains six complete alternating 200 ms samples for each
R9 group. `samples.csv` preserves group, baseline/current source, round,
source-cycle, matrix execution order, benchmark, CPU label, and raw
sample filename. `metadata.json` records revisions, hashes, toolchain,
platform, CPU, sampling settings, normalizations, and dropped rows.

## Coverage

| group | samples | distinct paired rows |
| --- | ---: | ---: |
| `r9-public` | 912 | 76 |
| `r9-format` | 48 | 4 |
| `r9-regex` | 120 | 10 |
| `r9-value` | 72 | 6 |
| `r9-stream` | 84 | 7 |
| `r9-duration` | 24 | 2 |
| `r9-validate` | 168 | 14 |
| `r9-concurrent` | 24 | 2 |
| `r9-namespace` | 144 | 12 |
| `r9-lex` | 72 | 6 |
| `r9-uri` | 144 | 12 |

The validate group maps baseline `/direct_match` and `/direct_miss`
to current `/compiled_match` and `/compiled_miss`; baseline generic
adapter rows are dropped. No other row mismatch is accepted.

The raw `BenchmarkPublishedRawUnionLateMember` value row is retained
as a diagnostic because the baseline returns bool+error while the
current API returns typed `Value`+error. The public
`BenchmarkSessionValidateUnionLateMember` row is a matching API
workload and is included as a normal paired row.

Dropped baseline rows: 12. Distinct row identities: 151.

## Review boundary

The `raw` directory preserves aggregate benchmark outputs; `comparisons`
contains benchstat results, including a normalized validation comparison.
`paired-geomeans.json` reports ratios of per-workload medians, with zero-valued
allocation rows explicitly counted and excluded. The five comparable value
rows exclude the diagnostic raw-union API; the normalized identity rows remain
included. `schema-text-confirmation` preserves the additional six alternating
1 s samples and their metadata. See the final report for interpretation and
the scope of the retained-memory measurements.
