# Architecture review iterations

Starting revision: `d497d859` on `codex/fresh-rewrite`.
Final production revision: `6842b63c`; the following commit records
architecture and review evidence only.

This ledger records the requested review from datatype foundations through XML
streaming. [ARCHITECTURE.md](../ARCHITECTURE.md) remains the sole architecture
authority. A concern finishes after three consecutive passes without a material
finding. An accepted finding resets that concern's count; changes also reopen
affected adjacent contracts. Cosmetic preferences and speculative abstractions
do not count as findings. Clean reviews establish bounded confidence, not proof
that no future improvement exists.

Each pass traces the owning types, constructors/transitions, readers and writers,
consumer contracts, failure/cleanup, resource limits, and tests. Later passes
challenge adversarial caller shapes and deletion opportunities instead of merely
repeating the first scan. The lead adjudicates findings and owns integration.

Go language-server tools were unavailable during discovery. Review uses scoped
source inspection, `go doc`, compiler diagnostics, and focused executable probes
where a concrete uncertainty requires them.

| Concern | Consecutive clean passes | State |
| --- | ---: | --- |
| Basic datatypes and value representations | 3 | Complete |
| Facets, simple derivation, and regex | 3 | Complete |
| Schema compilation and immutable execution tables | 3 | Complete |
| Runtime assessment and session lifecycle | 3 | Complete |
| Identity constraints and document identities | 3 | Complete |
| XML stream and namespaces | 3 | Complete |
| Public/source/diagnostic boundaries | 3 | Complete |

The final value-to-validation round comprises three independent read-only
passes over all four reopened concerns (datatypes, facets/regex, runtime, and
identity): foundation/consumer ownership, admission/error/opposing-shape
behavior, and compiler-literal-to-fixed/identity contracts. All three concluded
without an accepted material finding. The last identity candidate was withdrawn
after checking the normative field-value rule and the complete assessment path.
Schema's final passes challenged private publication, names, content execution,
and callers. XML's final passes covered borrowing, terminal I/O, namespaces,
reset/detach, and compile/validate/format consumers. Source's final passes
challenged acquisition cleanup, resolver contexts, typed nils, joined failures,
and diagnostic presentation. Each of these three concerns reached three clean
passes after its last relevant correction.

## Findings and decisions

Public/source/diagnostic pass 1: no material finding. Traced public option/source
conversion; source opening, bounded reads, repeatable finishing and cleanup;
resolver ownership and raw versus identity projections; loader acquisition
classification; immutable diagnostics, cause preservation, aggregation, and
location decoration. Existing tests cover zero/typed-nil sources, overflow probes,
joined failures, partial reads, stable finish, resolver contexts, and diagnostic
aliasing. Concrete facade adaptation and distinct URI projections preserve real
contracts; deleting them would move their work into callers.

### Accepted findings

- Fixed simple elements compared canonical text instead of typed equality,
  rejecting equivalent duration spellings (`P1D` / `PT24H`). The runtime owner
  now requests identity and compares values; untyped mixed text remains lexical.
- Name-table construction accepted seed orders that assigned a nonempty URI to
  namespace ID zero, while lookup assumes that ID means no namespace. The owner
  now seeds the empty namespace first and rejects invalid publication shape.
  Regression tests failed on the original code and pass with the fix; name-budget
  and frozen-view behavior are covered.
- Recursive identity tables conflate local entries with entries propagated from
  child scopes. The owning table model now preserves origin so local keys win
  over child conflicts while sibling ambiguity and true local duplicates retain
  their required behavior.

- Value-builder capacity reservation bypassed the storage budget and could leave
  the program reading a stale completion slice. Capacity admission charges
  the owning budget before growth; construction and evaluation now share one
  authoritative completion representation.
- Removing a custom file resolver changed the context identity despite retaining
  the same built-in fallback, causing duplicate graph traversal and budget use.
  Known absent resolvers now normalize to the built-in context.
- Public diagnostic traversal accepted typed-nil aggregate pointers and could
  panic while unwrapping them. The diagnostic boundary now consistently admits
  its own absent value and pointer forms, including nested wrappers.

- XML input-limit probing charged the unadmitted overflow byte to token source
  offsets, shifting valid prefix spans. The byte-stream owner now counts admitted
  source bytes for offsets while retaining the terminal limit and I/O causes.
- Parser detach retained oversized scratch until a future reset. The parser now
  owns one buffer cleanup operation shared by reset and detach; real oversized
  token tests prove immediate release and ordinary small-buffer reuse.
- A union enumeration did not request structural list items from its selected
  member, so matching list-valued literals were rejected. Enumeration demand
  now propagates through member evaluation before structural comparison.
- Per-source limit diagnostics lost their source path when joined with read or
  close errors before the compiler tried to decorate them. Location belongs on
  the direct diagnostic at creation, before wrapper-preserving aggregation.

- Borrowed-byte validation restarted its work counter on normalized fallback.
  Both attempts now share one counter. Requested canonical/identity projections
  bypass unprojected fast evaluation; builtin unprojected strings keep their
  allocation-free path under the same lexical work limit.
- Queued builder records could pay storage admission twice during sealing.
  Queued records prepay their complete estimate; reserved incomplete slots pay
  only the remaining metadata at completion. Failed admission remains retryable
  without uncharged growth or duplicate fixed charges.
- Enumeration literal construction suppressed member/item facets along with its
  containing facets, changing union member selection. Only the containing type's
  facets may be deferred; nested members and list items always enforce theirs.
- Fixed ordered-facet derivation selected the oldest inherited bound rather than
  the nearest declaration of the same kind. Reverse lookup preserves the full
  sequence needed by partially ordered values while applying the fixed rule.
- g* calendar values compared lexical clock components despite retaining a
  normalized instant. Their identity now uses that instant plus timezone
  presence, including offsets crossing the reference-day boundary.
- Unprefixed NOTATION values could bypass declaration checks without a resolver.
  One expanded-name assignment path now serves QName and NOTATION; NOTATION
  always checks the resulting declaration.
- A hot validation predicate cloned complete type views, union members, and
  enumeration metadata to answer whether a type was an unconstrained string.
  A scalar query owned by the value program now answers that exact question;
  the unused schema enumeration helpers were deleted.
- Repeated regex category terms rebuilt Unicode sets and repeatedly copied
  intermediate unions. Parser-local catalog caching, contiguous unit-stride
  conversion, and a heap merge bound intermediate storage by live inputs and
  output size. No global cache or alternate matcher was introduced.

### Rejected candidates

An explicit source identity also returned by a resolver consumes both an explicit
source-descriptor slot and a distinct resolver-identity slot. This is deliberate:
ARCHITECTURE.md defines the budget as their sum, not the size of their union.
The observed limit rejection follows that binding contract; no change is needed.

A missing compiled-model table could be manufactured through test-only mutable
build aliases. Production installs aligned models immediately before private
publication. No reachable writer violates that invariant; a duplicate projection
audit would conflict with the publication contract.

A fixed inclusive bound does not prohibit a later exclusive bound. The fixed
rule constrains the value of the same facet, while the additional bound can
narrow the value space. The inherited-bound representation preserves both;
the XSD 1.0 datatype contract and a libxml2 comparison support this behavior.

Standard-library traversal of a manually fabricated nil aggregate pointer, or
an external wrapper containing one, is outside direct diagnostic admission.
Constructors normalize known direct absent forms and `xsderrors` traversal
handles them; rewriting arbitrary external wrapper chains would break cause
identity and the explicit wrapper-preservation contract.

A valid attribute field remains qualified when the selector element has an
unrelated content error. XSD 1.0's
[identity-constraint rule](https://www.w3.org/TR/xmlschema-1/#cvc-identity-constraint)
qualifies field values; it does not require the selector's entire content to be
valid. Current-scope handling already applies incoming element invalidity to
applicable element fields. Ancestor handling adds newly discovered scope
invalidity. Repeating the incoming flag there would be redundant and cannot
invalidate the attribute-field reproduction. The reviewer withdrew this finding.

### Prior art

[The JSON v2 comparison](xmlstream-jsonv2-comparison.md) found no material reason
to split the stream owner or add interfaces/whole-element values. Its result is
separate from the three XML contract-review passes.

## Verification

Final production verification for `6842b63c` passed on Go 1.27.0,
Darwin/arm64, Apple M2 Max:
`make test`, `make race`, `make wasm-test`, `make web-test`,
`make browser-test`, `make fuzz-smoke`, `make bench-smoke`,
`make staticcheck`, and `make lint`. Changed Go files are formatted and
`git diff --check` passes. The browser suite passed all five tests. The earlier
race build interruption came from a removed review probe; its clean rerun
passed. No temporary probe remains in the source tree.

Behavior tests cover typed fixed values, g* timezone identity, union/list
selection and facets, nearest fixed bounds, storage admission/failure/retry,
shared raw-fallback work, namespace-zero construction, local identity-key
precedence, source resolver normalization, joined-error locations, absent
diagnostics, admitted XML offsets, immediate detach cleanup, and regex range
construction. These include public compiler/validation regressions in addition
to owner-level tests. Public usage and interfaces remain consistent with README;
no new public API or configuration is introduced.

### Performance method

Compare the final changes against `d497d859` using identical fixtures in a
detached baseline checkout. Run six 200 ms samples per case, serially, alternating
baseline/current order on each sample; use `bin/benchstat` for comparisons.
XML and identity-propagation measurements were taken after those implementations
froze. Public validation, compiler, regex construction, and metadata-query
measurements use freshly built final binaries. Allocation counts are cumulative
per operation, not peak retained memory. Timing comparisons reflect this host
and workload; they are not universal speedup guarantees.

### Results and accepted costs

| Workload | Before → after | Assessment |
| --- | --- | --- |
| Compile 128 repeated Unicode category terms | 148.77 → 5.00 ms; 843.9 MiB → 117.8 KiB allocated | Reusing canonical ranges removes repeated Unicode expansion |
| Public regex-category schema compilation | 2.414 → 1.738 ms; 7.934 → 6.454 MiB | 28.0% faster, 18.7% fewer allocated bytes; allocations increase 5.5% |
| Merge 512 repeated sets of 128 ranges | 3.171 → 4.735 ms; 2072 → 9.99 KiB; 2044 → 9 allocations | Accept 49.3% more time for bounded live-input/output storage; public category compilation improves |
| String predicate, builtin / union / faceted | 60.7 / 69.1 / 117.1 → 2.19 / 2.52 / 2.83 ns | All now allocate zero; old union/faceted paths allocated 8/72 bytes |
| QName and NOTATION, 128 repeated values | No significant time change; zero allocations retained | Opposing resolver path remains stable |
| Seven XML parser shapes | No significant time change; identical allocations | Covers references, wide lazy/materialized attributes, text, mixed tokens, and CDATA boundaries |
| Identity-table propagation, depths 16–256 | No significant time change; 12.0–13.9% more bytes; same allocation counts | Origin metadata is required for local-key precedence |
| Public identity rows / nested selections | Mostly unchanged time; 5.4–18.1% more bytes; same allocation counts | Retained provenance increases entry size |
| Small schema compilation | 82.24 → 85.66 µs; 569 → 573 allocations | 4.2% time increase accepted for corrected boundary admission |
| Deep type chains / repeated union compilation | No significant time change; same allocation counts | Construction-budget fixes preserve scaling |

Typed fixed-value checks request identity to implement value-space equality.
For 128 fixed items per validation, allocations change as follows:

| Type | Allocated bytes before → after | Allocations before → after | Timing |
| --- | ---: | ---: | --- |
| string | 0 → 1024 | 0 → 128 | 1.9% faster |
| integer | 2048 → 3072 | 256 → 384 | No significant change |
| decimal | 512 → 2048 | 128 → 256 | 4.9% faster |
| duration | 410 → 4779 | 128 → 512 | 3.9% slower |
| date | 8197 → 10246 | 512 → 640 | 5.9% faster |
| gDay | 7172 → 15369 | 512 → 896 | 20.8% slower |

These costs are explicit correctness trade-offs. Text-only comparison cannot
implement duration or timezone-normalized g* equality. A second runtime value
parser or datatype-specific comparison policy in validation was rejected; the
value owner remains authoritative. The scalar-query change offsets metadata
copying elsewhere without changing that equality contract.

Reproduce the public subset with
`go test -run '^$' -bench 'Benchmark(Compile(SmallSchema|DeepSimpleTypeChain|RepeatedNestedUnionMembers|RegexCategoryEscapes)|SessionValidate(IdentityConstraintsRows|NestedIdentitySelections|FixedSimpleValues|QualifiedSimpleValues))$' -benchtime=200ms -benchmem .`.
The regex subset uses
`go test -run '^$' -bench 'Benchmark(CompileRepeatedCategoryTerms|UnionManyRepeatedRanges)$' -benchtime=200ms -benchmem ./internal/xsdregex`.
Current scalar-query behavior, allocations, and benchmark are retained in
`internal/value/metadata_test.go`. The paired query fixture uses each revision's
predicate against identical builtin, union, and facet shapes.

Raw logs, paired fixtures, the runner, and `benchstat` output are retained locally
under `.lab/rewrite/review-iterations/`; this ledger records the durable results.

