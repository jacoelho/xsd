# Current rewrite versus main

Measured 2026-09-06: committed main `cc94656a` versus rewrite `bb9045cd`.
These revision-pinned results are separate from the older
[rewrite-baseline comparison](rewrite-performance.md).

82 matching public workloads, six alternating samples of 200 ms per workload, one CPU, Go 1.27.0 on Darwin/arm64 (Apple M2 Max). Compilation and validation succeeded for all benchmark inputs on both revisions. Production code was unchanged; the isolated main checkout received identical benchmark fixtures. This measures the locally available main commit, not an independently refreshed remote branch.

Geometric mean of workload median time ratios: **-27.74%** overall, **-41.32%** for 23 compilation workloads, and **-21.62%** for 59 validation workloads. These are equally weighted workloads, not an assumed production traffic mix.

| Workload | Main | Current | Time change |
| --- | ---: | ---: | ---: |
| Small-schema compilation | 200.32 µs | 105.88 µs | -47.14% |
| Deep type-chain compilation | 8571.42 µs | 4281.87 µs | -50.04% |
| Repeated QName validation | 169.94 µs | 76.71 µs | -54.86% |
| Regex-category schema compilation | 2945.10 µs | 1866.39 µs | -36.63% |
| 16 MiB streamed schema compilation | 229065.17 µs | 95086.74 µs | -58.49% |
| Identity rows (1,000) | 2836.74 µs | 2621.40 µs | -7.59% |
| Fixed gDay values (128) | 261.93 µs | 185.12 µs | -29.33% |
| Repeated small-document session | 280.02 µs | 278.60 µs | -0.51% |
| Schema-text compilation | 333.44 µs | 350.83 µs | +5.22% |
| Substitution-group compilation | 692.20 µs | 699.93 µs | +1.12% |

The repeated small-document difference is within noise. Schema-text compilation is 5.2% slower by median but not significant (p=0.699); substitution-group compilation is 1.1% slower (p=0.041). Some samples have substantial host-noise outliers; medians and alternating order limit their effect but do not establish universal speedups.

## Allocation trade-offs

- A 16 MiB streamed schema allocates 36,381,472 → 125,760 bytes (34.70 MiB → 122.8 KiB).
- Small-schema compilation allocates 3.3% fewer bytes and 8.3% fewer objects.
- Deep type-chain compilation allocates 4.5% more bytes and 52.1% more objects, while taking half the time.
- Identity validation at 1,000 rows allocates 8.3% more bytes; nested identity selection at depth 256 allocates 17.2% more. Allocation counts fall slightly in both.
- The 128-item fixed-gDay workload allocates 5,122 → 15,361 bytes and 384 → 896 objects, while taking 29.3% less time. Fixed string/decimal/duration values gain identity-projection allocations where main has zero reported allocations.

An overall byte-allocation ratio is sensitive to near-zero baseline measurements in warmed fixed-value workloads. The nonzero-pair geometric mean is +10.1% over 76 rows; it is not a peak-memory measure or a universal memory result. Allocation-count geometric mean is -10.5% over 73 nonzero pairs. The explicit per-workload trade-offs above are more informative.

## Evidence and scope

- [Paired statistical comparison](benchmark-evidence/main-2026-09-06/comparison.txt)
- [Per-workload medians and ratios](benchmark-evidence/main-2026-09-06/results.json)
- [Run metadata](benchmark-evidence/main-2026-09-06/metadata.json)
- [Main samples](benchmark-evidence/main-2026-09-06/main.txt) and
  [rewrite samples](benchmark-evidence/main-2026-09-06/current.txt)

The metadata records the exact revisions, toolchain, benchmark filter, binary
hashes, and fixture hashes. To reproduce, use isolated checkouts of those
revisions and copy the six named benchmark files from the rewrite to main.
Build each public test binary with `go test -c -o public.test .`, then invoke
it with `GOMAXPROCS=1`, `-test.cpu=1`, `-test.run=^$`,
`-test.benchtime=200ms`, `-test.benchmem`, and the recorded benchmark filter.
Alternate main/rewrite order for six rounds, exclude the two unmatched rows
listed below, and compare outputs with `benchstat`.

Two current-only source-document benchmark rows were excluded because their separate fixture was not installed in the main checkout. They are not classified as unsupported. The matched rows use identical source fixtures on both revisions. This comparison does not measure internal formatter, tokenizer, or regex-matcher microbenchmarks separately.

## All matched workloads

| Workload | Time change | Main B/op | Current B/op | Main allocs/op | Current allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| CompileAndFirstSession | -47.14% | 280045 | 273197 | 627 | 575 |
| CompileAttributeGroupFanout/refs_1 | -59.82% | 165040 | 133520 | 416 | 338 |
| CompileAttributeGroupFanout/refs_10 | -43.05% | 204664 | 172200 | 756 | 644 |
| CompileAttributeGroupFanout/refs_100 | -13.47% | 655289 | 611128 | 3564 | 3092 |
| CompileAttributeGroupFanout/refs_1000 | -3.90% | 5.0757e+06 | 5.125e+06 | 30004 | 27428 |
| CompileChameleonTargetFanout/targets_1 | -61.15% | 313368 | 132568 | 489 | 340 |
| CompileChameleonTargetFanout/targets_32 | -46.44% | 2.9315e+06 | 427241 | 3189 | 1703 |
| CompileChameleonTargetFanout/targets_8 | -53.50% | 906026 | 197864 | 1148 | 696 |
| CompileCountedChoiceDFA/large | -37.29% | 360258 | 331753 | 2331 | 2149 |
| CompileCountedChoiceDFA/medium | -42.46% | 259685 | 231716 | 1402 | 1263 |
| CompileCountedChoiceDFA/small | -49.05% | 193546 | 165128 | 752 | 661 |
| CompileDeepSimpleTypeChain | -50.04% | 2.26838e+06 | 2.3696e+06 | 8478 | 12899 |
| CompileDuplicateSchemaSources | -49.77% | 1.23777e+06 | 221816 | 894 | 444 |
| CompileIncludeGraph | -43.15% | 7.05295e+06 | 767392 | 6821 | 3623 |
| CompileOpaqueAnnotationPayload | -19.15% | 361608 | 258176 | 2874 | 1279 |
| CompileRegexCategoryEscapes | -36.63% | 6.69372e+06 | 6.76692e+06 | 9768 | 5715 |
| CompileRepeatedNestedUnionMembers | +2.51% | 830650 | 810944 | 4784 | 5228 |
| CompileSchemaText | +5.22% | 1.14056e+06 | 1.37958e+06 | 360 | 282 |
| CompileSmallSchema | -47.14% | 206306 | 199458 | 625 | 573 |
| CompileStreamingSource/1048576B | -59.96% | 2.38467e+06 | 125760 | 376 | 275 |
| CompileStreamingSource/16777216B | -58.49% | 3.63815e+07 | 125760 | 383 | 275 |
| CompileStreamingSource/4096B | -62.25% | 167064 | 125760 | 361 | 275 |
| CompileSubstitutionGroups | +1.12% | 460256 | 547096 | 1621 | 1547 |
| SessionValidateAllContent/1024 | -87.76% | 1960 | 1960 | 513 | 513 |
| SessionValidateAllContent/16 | -13.82% | 0 | 0 | 0 | 0 |
| SessionValidateAllContent/256 | -64.83% | 0 | 0 | 0 | 0 |
| SessionValidateAllContent/4 | -6.13% | 0 | 0 | 0 | 0 |
| SessionValidateAllContent/64 | -33.13% | 0 | 0 | 0 | 0 |
| SessionValidateDateDecimalRows | -9.93% | 34 | 33 | 1 | 1 |
| SessionValidateDeepThenShallow | -6.61% | 32 | 32 | 1 | 1 |
| SessionValidateDisjointIdentityPaths/expanded | -1.25% | 159129 | 155217 | 337 | 302 |
| SessionValidateDisjointIdentityPaths/expanded_distinct_control | -2.55% | 550936 | 539970 | 32445 | 32343 |
| SessionValidateDisjointIdentityPaths/expanded_distinct_namespaces | -1.39% | 3.3485e+06 | 3.3415e+06 | 32998 | 32909 |
| SessionValidateDisjointIdentityPaths/lexical | -12.25% | 180278 | 176458 | 303 | 280 |
| SessionValidateExpandedIdentityPaths | -5.03% | 359797 | 396151 | 2528 | 2521 |
| SessionValidateFixedSimpleValues/date | -41.81% | 6146 | 10241 | 384 | 640 |
| SessionValidateFixedSimpleValues/decimal | -51.74% | 1 | 2048 | 0 | 256 |
| SessionValidateFixedSimpleValues/duration | -47.15% | 1 | 4779 | 0 | 512 |
| SessionValidateFixedSimpleValues/gDay | -29.33% | 5122 | 15361 | 384 | 896 |
| SessionValidateFixedSimpleValues/integer | -63.94% | 1026 | 3072 | 128 | 384 |
| SessionValidateFixedSimpleValues/string | -25.50% | 1 | 1024 | 0 | 128 |
| SessionValidateIDAttributeStart | -15.96% | 840 | 840 | 101 | 101 |
| SessionValidateIdentityConstraintsRows/rows_10 | -6.65% | 4168 | 4392 | 42 | 42 |
| SessionValidateIdentityConstraintsRows/rows_100 | -7.85% | 37725.5 | 40240 | 321 | 321 |
| SessionValidateIdentityConstraintsRows/rows_1000 | -7.59% | 445636 | 482525 | 4021 | 4016 |
| SessionValidateNamespaceAdmissionChurn/depth_16/churn_false | -9.12% | 32 | 32 | 1 | 1 |
| SessionValidateNamespaceAdmissionChurn/depth_16/churn_true | -2.03% | 32 | 32 | 1 | 1 |
| SessionValidateNamespaceAdmissionChurn/depth_256/churn_false | -12.14% | 32 | 32 | 1 | 1 |
| SessionValidateNamespaceAdmissionChurn/depth_256/churn_true | -9.30% | 32 | 32 | 1 | 1 |
| SessionValidateNamespaceAdmissionChurn/depth_64/churn_false | -8.60% | 32 | 32 | 1 | 1 |
| SessionValidateNamespaceAdmissionChurn/depth_64/churn_true | -4.63% | 32 | 32 | 1 | 1 |
| SessionValidateNestedIdentitySelectionPaths | -3.32% | 131264 | 131248 | 3 | 3 |
| SessionValidateNestedIdentitySelections/depth_16 | -11.15% | 4122 | 4601 | 27 | 27 |
| SessionValidateNestedIdentitySelections/depth_256 | -12.30% | 71191.5 | 83411.5 | 279 | 277 |
| SessionValidateNestedIdentitySelections/depth_64 | -8.63% | 17167.5 | 19680 | 79 | 79 |
| SessionValidateQualifiedSimpleValues/NOTATION | -69.58% | 2 | 0 | 0 | 0 |
| SessionValidateQualifiedSimpleValues/QName | -56.40% | 1 | 0 | 0 | 0 |
| SessionValidateRepeatedLaxWildcard | -2.18% | 32 | 32 | 1 | 1 |
| SessionValidateRepeatedQNameValues | -54.86% | 33 | 32 | 1 | 1 |
| SessionValidateRepeatedSmallDocument | -0.51% | 35 | 34 | 1 | 1 |
| SessionValidateRepeatedXSIType | -3.81% | 33 | 32 | 1 | 1 |
| SessionValidateRetainedIdentityPaths | -7.13% | 651720 | 688085 | 4024.5 | 4018 |
| SessionValidateSharedExpandedIdentityPrefix/control | -0.02% | 5249 | 4498 | 19 | 12 |
| SessionValidateSharedExpandedIdentityPrefix/identity | -8.71% | 4.7436e+06 | 4.73737e+06 | 19811 | 19740 |
| SessionValidateSimpleIDEnd | -19.90% | 832 | 832 | 101 | 101 |
| SessionValidateStringLengthFacet | -40.49% | 32 | 32 | 1 | 1 |
| SessionValidateUndeclaredRootXSIType | -6.38% | 32 | 32 | 1 | 1 |
| SessionValidateUnionLateMember | -5.07% | 32 | 32 | 1 | 1 |
| SessionValidateWideChoice | -9.56% | 401 | 247 | 5 | 3 |
| ValidateDeeplyNestedDocument | -12.44% | 184288 | 159456 | 311 | 174 |
| ValidateDuplicateAttributes | -1.97% | 612000 | 586985 | 1604 | 1083 |
| ValidateIdentityConstraints | -7.09% | 172344 | 169648 | 610 | 501 |
| ValidateIdentityConstraintsFields/fields_1 | -12.80% | 124920 | 122176 | 473 | 363 |
| ValidateIdentityConstraintsFields/fields_3 | -15.34% | 165816 | 156448 | 1190 | 876 |
| ValidateIdentityConstraintsFields/fields_8 | -18.91% | 232080 | 211224 | 2424 | 1891 |
| ValidateIdentityConstraintsRows/rows_10 | -11.81% | 87048 | 86736 | 130 | 114 |
| ValidateIdentityConstraintsRows/rows_100 | -6.94% | 172344 | 169648 | 610 | 501 |
| ValidateIdentityConstraintsRows/rows_1000 | -6.45% | 1.05783e+06 | 1.07231e+06 | 5149 | 4626 |
| ValidateManyRecoverablePathErrors | -5.10% | 110352 | 110080 | 631 | 627 |
| ValidateRepeatedSmallDocument | +1.41% | 76808 | 76080 | 38 | 28 |
| ValidateSmallInvalidDocument | -23.02% | 77840 | 77096 | 59 | 48 |
| ValidateSubstitutionGroup | -6.94% | 78272 | 76840 | 60 | 37 |
