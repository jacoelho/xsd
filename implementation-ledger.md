# Plan implementation evidence

> Historical record imported with baseline commit `2764e554`. Its completion
> statements concern the earlier architecture task. The active rewrite is tracked
> in [rewrite-plan.md](rewrite-plan.md); [ARCHITECTURE.md](ARCHITECTURE.md) owns
> the current architecture.

Status: complete, 2026-09-05. Earlier checkpoint statuses are historical; the
release closure below supersedes their pending gate/review notes.

## Requirement lock and baseline

Required: implement the accepted `plan.md` P0–P3 and D1–D10; preserve I1–I10;
iterate with three independent adversarial reviewers; pass the repository's full
verification contract and focused allocation/benchmark comparisons. No public
behavior change, alternate publication path, or compatibility wrapper is selected.

The accepted plan is preserved verbatim in
`/tmp/xsd-implementation-20260905/baseline/plan.md`. Its historical planning charter
remains intact; implementation status and evidence are appended here and linked
from the plan. Root is the sole writer. Reviews are read-only.

Baseline: HEAD `72178fe6` plus all existing dirty changes, captured before writes.
The 40,093-file manifest has SHA-256
`8c0d96958d8f1bb821607005acece35a6c19fb6ee161cab9a55445094ce01b0a`.
Snapshots, initial diff/status, command logs and measurements reside under
`/tmp/xsd-implementation-20260905/`. Unrelated changes are preserved.

The existing accepted design closes the decision gate. This standard full-mode
pass revalidates the current seams, implements with one writer, then performs the
three requested adversarial reviews. A material contradiction reopens the relevant
decision; a routine rename or fixture choice does not change the design.

## P0 evidence

- `go_workspace` confirmed the module; `RootStart` semantic references still show
  one production caller and two direct tests.
- `go test ./internal/validate ./internal/runtime ./internal/compile .` passed
  against the unchanged source baseline (`baseline-tests.log`).
- The existing repeated-XSI benchmark exercises declared children. A focused
  undeclared-root-XSI benchmark is added before measuring either implementation,
  so the changed branch is compared with identical fixtures.

## Implementation and amendments

Implementation is in progress. No design amendment has been required.


### P1 checkpoint

- Deleted RootInput/StartResult/RootStart/rootTypeFromXSIType and their direct
  flag tests. Root selection now returns schemaStart, with a private transient
  type origin consumed by common assessment. Deleted the now-unreferenced
  qnameResolverForAttrs helper; ordinary callers retain the same QName owner.
- Added public diagnostic-order/identity/reuse/MaxErrors tests. They pass both
  current source and the captured baseline through a Go overlay
  (`p1-oracles-baseline.log`). The complete validation/root suites pass
  (`p1-tests.log`), and lint reports zero issues (`p1-lint.log`).
- Six samples per benchmark: all existing validation cases show no significant
  time change and no increased allocations. Undeclared-root XSI is 5.60% faster
  (p=0.002, n=6), with 32 B/op and 1 alloc/op unchanged. This is a local result,
  not a general throughput guarantee (`validation-p1-comparison.txt`).
- ARCHITECTURE.md records the selected state, ordering, identity distinction and
  rejected flag protocol. P1 is ready for independent final review.

### IM1 — Published accessor test placement

The plan predicted a runtime-package publication fixture. Reinspection shows that
PublishSchema requires the complete builtin declaration set; constructing it
there would duplicate compiler initialization. Place TestPublishedElementValueConstraints
in the existing compile_test publication suite using mutableSchemaBuild and
publishSchema. It still observes the exact PublishSchema → Schema accessor seam,
including successful consumption. Runtime retains direct packed/projection audit
and alias tests. This changes the predicted test file, not ownership, behavior,
production dependencies or required proof. Existing authorization covers this
fixture correction; the original plan remains in the baseline snapshot.


P2 rename tooling: gopls returns unified semantic edits against cached file
contexts. The next rename's context still referenced the preceding exported
spelling, so git apply correctly rejected it before writing. The application
script now maps only gopls's exact changed spans onto the current lines, rejecting
any overlap or line-count change. It does not substitute textual symbol search
or blanket replacement for semantic rename. All edits remain reviewable against
the captured source baseline.


### P2 checkpoint

- Applied 47 semantic renames to the finite publication inventory; retained the
  compiler's real semantic reads and the isolated identity-state fixture seam.
- Deleted six obsolete element-value slice construction/comparison/lookup
  functions, the unused element-start shape/constructor/comparer, three unused
  read facades and the fourth facade after migrating its two real consumers.
- Published accessor tests cover unconstrained/fixed/default/empty-default,
  sparse declaration IDs, shared source records, simple/simple-content/mixed
  owners, canonical values, start facts, NoElement and out-of-range IDs. Mutating
  retained source aliases leaves both global and ID-based reads unchanged.
- The fixed/default discriminant is now corrupted in both the direct packed audit
  and the complete projection-audit route. The latter first proves its uncorrupted
  fixture is accepted. Existing publication failure/consume/retry/work tests remain.
- All four affected package suites pass (`p2-tests.log`); lint reports zero issues
  (`p2-lint.log`). A fresh `gopls check` passes (`p2-fresh-semantic.log`); the MCP
  server's cached test diagnostics are stale and are not completion evidence.
- The earlier `p2-renames-test.log` is superseded: its run overlapped deletion of
  export_test.go and reported that vanished input. Subsequent checks ran after
  source writes stopped. No failing result is used as verification evidence.
- ARCHITECTURE.md now records private projection mechanics and the sole packed
  element-value representation, with independent audit retained.


### IM2 — Final review repairs (P2/D3 and P3/D1)

The behavior adversary found no material defect. The scope adversary found the
retired RootStart name still allowed by the borrowed-attribute boundary checker.
The publication adversary found GlobalTypeByName still exported after its
TypeDerivationRead parameter became private. Semantic references show only
Schema.GlobalType and two runtime tests consume that helper.

Both findings were accepted. Remove the obsolete allowlist entry and semantically
rename GlobalTypeByName to globalTypeByName, including its production/test callers.
This adds one private helper to D3's finite inventory (48 renames total) and one
predicted test-enforcement file to P3. The original inventory is retained as
historical baseline evidence. No owner, behavior, budget, public interface, or
representation changes. These repairs follow the authorized deletion/private
publication destination; no waiver or additional approval is required.

The post-repair focused command `go test ./internal/runtime ./internal/compile
./internal/validate ./tests .` passed (`closure-focused.log`). Fresh gopls checks
of the four repaired files passed (`closure-semantic.log`). Full gates and final
review closure are pending at this checkpoint.

### P3 structural and behavior evidence

- `final-inventory.log`: Go AST parsing of all 299 current Go files, including
  test and build-tagged files, finds no identifier using any of 66 retired
  spellings and finds all 48 private replacement declarations. The deleted
  runtime/export_test.go file is absent. This supplements semantic renames and
  current compiler checks; string literals naming old helpers in diagnostic text
  are not executable references. The obsolete RootStart allowlist string is gone.
- `final-renames.json` retains each old/new symbol and defining file. The finite
  D1–D10 obligations are covered by the AST inventory, semantic migrations and
  task-only `task.diff`. Shared semantic read types and the isolated
  ElementIdentityConstraintIDs fixture remain; the canonical packed audit remains.
- Controlled Go overlays demonstrate test sensitivity without changing the
  worktree. Moving hint rejection before valid root-XSI selection fails
  TestUndeclaredRootXSISelectionOrder/selected_type_before_root_hint with an
  unexpected unsupported.xsi_schema_location diagnostic (`mutant-root-hint.log`).
  Disabling the independent packed-value scalar audit fails
  TestProjectionAuditRejectsElementConstraintCorruption with "projection audit
  accepted fixed/default corruption" (`mutant-packed-audit.log`). Both mutations
  compiled and failed the intended assertion. These are expected negative proofs.
- The public identity/root-selection tests also pass with original production
  files supplied through the captured baseline overlay. Explicit error tuples are
  behavior-preservation oracles, not expectations copied from the new path.
- `closure-source-audit.json` identifies all task changes against the saved dirty
  baseline, with zero unexpected changes. Root APIs/options, README, examples,
  CLI/WASM/worker code, source/stream/namespace owners and dependency files match
  that baseline. Existing user edits remain in those files. The only previously
  dirty production file further changed here is session_runtime.go; its task diff
  contains the selected root/XSI change. ARCHITECTURE.md receives the two selected
  ownership writebacks without replacing the user's existing architecture edits.

### Final measurements

Raw six-sample baseline/final results and benchstat reports are in the evidence
folder: validation-before.txt, validation-final.txt,
validation-final-comparison.txt, compile-before.txt, compile-after.txt and
compile-comparison.txt. Runs used Go 1.27.0, darwin/arm64, Apple M2 Max, 12-way
execution, one-second samples, identical fixtures, and serial benchmark processes.

Repeated small-document, repeated declared XSI, QName, lax wildcard and wide-choice
validation show no significant time or allocation regression. The undeclared-root
XSI case improves from 1.589 to 1.488 microseconds (-6.33%, p=0.004, n=6), with
32 B/op and one allocation unchanged. Wide-choice amortized scratch varies around
104 B/op; its distributions do not show regression. Compile small-schema,
substitution, deep simple-type chain and repeated nested-union cases show no
significant time/B change; allocation counts remain 625/1621/8478/4784.

The closure repairs change one helper spelling and delete one test permission;
they do not alter executable work, storage, or benchmarks, so these focused
comparisons remain applicable. The full smoke benchmark is rerun after repairs.
This is local measurement evidence, not a general throughput guarantee.

### Requirement and invariant traceability

| Contract | Implementation and proof | Status before release closure |
| --- | --- | --- |
| P0 | Dirty-baseline manifest/snapshot, semantic inventory, baseline-tests.log, original-source test overlay and six-sample baseline measurements. | Proved |
| P1; D1/D2/D8 | Session-owned schemaStart, transient origin consumed in common assessment; old protocol/flag tests deleted; public root/identity tests, transaction/recovery/reset suites, baseline comparisons. | Proved |
| P2; D3–D10 | 48 private renames, six slice APIs and start-construction protocol deleted, four facades deleted/migrated; published accessor and independent corruption/lifecycle tests. IM1/IM2 record exact forecast amendments. | Proved |
| P3 | Full deletion inventory, unchanged public/adapter sources, strengthened stream allowance, architecture writeback, focused checks and task-only source audit. | Final gates/re-review pending |
| I1 | Full stream, compile, public XML-document and conformance suites; supported-input policy files unchanged by this task. | Proved by existing final-gates.log; post-repair rerun pending |
| I2/I3 | Source acquisition/resolver/alias tests, public shape and phase-import enforcement; source/public API files unchanged. | Proved by existing final-gates.log; post-repair rerun pending |
| I4 | Publication source/projection corruption, alias isolation, failure preservation, consume, repair/retry and charged-work tests; independent audit retained. | Proved |
| I5 | session_start_transaction_test.go and session_error_limit_test.go cover rollback and syntax-only continuation; new public tests prove later XML failure and reuse. | Proved |
| I6/I7 | Existing start/wildcard/XSI/nil tests plus TestUndeclaredRootXSISelectionOrder preserve distinct modes, absent/false nil, fatal/recoverable order and literal diagnostic tuples. | Proved |
| I8 | Session overlap/reset/retention tests, full race gate and unchanged allocation distributions; transient origin never reaches a frame/schema/session cache. | Proved by existing final-gates.log; post-repair race rerun pending |
| I9 | TestInvalidSemanticXSITypeIdentityPreservesLexicalQName verifies unknown/non-derived versus malformed/unbound QName identity, error order, MaxErrors and reuse. | Proved |
| I10 | Unchanged admitting budgets/work accounting; limit/retention tests, four fuzz targets and opposing benchmark shapes; no new retained table, queue, goroutine or cache. | Proved by existing final-gates.log; post-repair smoke rerun pending |
| R1–R7 / accepted design B | Historical planning charter/designs retained; implemented P0–P3 preserves R5 contracts, writes architecture at its owner, and records separate implementation evidence. R4's planning review is not substituted for the three implementation reviews. | Release closure pending |

Architecture delta: one root-only transport protocol and a duplicate resolution
are removed; schemaStart gains a consumed operation-local origin. Runtime's
construction surface contracts and the alternative element-value representation
is deleted. No package/dependency/public option/worker/cache/compatibility path
is added. The remaining objection is scope: this is a bounded contraction, not
an opaque-builder rewrite. The accepted comparison rejects that broader rewrite
because its additional lifecycle/coupling does not resolve a present obligation.
There is no temporary production mechanism or deferred migration.


### Implementation adversarial review closure

| Reviewer | Initial finding | Repair re-review |
| --- | --- | --- |
| adversary_behavior | No material P1/I5–I9 defect; independently traced selection, XSI identity, rollback, stop and reuse. | Clear after IM2; current source and focused tests preserve those contracts. |
| adversary_publication | GlobalTypeByName remained exported with a private derivation parameter. | Clear after semantic rename; no remaining P2/D3–D10 projection, ownership, alias, audit or lifecycle gap. |
| adversary_scope | Deleted RootStart remained in the borrowed-attribute allowlist. | Clear after removal; original/amended plan, source inventory and task scope agree. Only post-repair gate completion/bookkeeping remained pending. |

All three reviews were independent, read-only and bounded; root applied both
repairs. No minority finding remains unresolved. The independent broad recheck
also retained the existing source/namespace, compiler phase, simple-value callback,
derivation and identity ownership decisions. An exploratory report based on stale
MCP test contents claimed the I9 regression was missing; current on-disk
TestInvalidSemanticXSITypeIdentityPreservesLexicalQName and the behavior reviewer
contradict that claim, so it was rejected rather than prompting a duplicate test.


### Release closure — 2026-09-05

The post-repair command
`make test race wasm-test web-test browser-test fuzz-smoke bench-smoke staticcheck lint`
completed with exit status 0 (tracked exec session 22190). The worker suite passed
17 tests and browser suite passed five; all four fuzz targets and the complete
benchmark-smoke selection passed. Staticcheck and lint passed, with zero lint
issues. This supersedes pre-repair final-gates.log as release evidence.

P0, P1, P2 and P3 are complete; all 25 plan tasks are checked. D1–D10, I1–I10,
public-contract preservation, original/amended plan conformance and the separate
three-review implementation requirement are proved by the mapped evidence above
and this post-repair gate run. No pending review, source repair, benchmark decision,
architecture migration, temporary production path or follow-up remains.

Formatting verification finds all task-changed Go files clean. The final source
audit compares the entire 40,093-file original manifest, checks every frozen Go
source plus architecture/dependency/build file, and records all task-owned changes
in final-source-audit.json and task.diff. Unchanged public/example/README/adapter
files preserve the original dirty baseline. git diff --check is the final text gate.

Raw evidence resides in `/tmp/xsd-implementation-20260905/`.
`evidence-hashes.json` records SHA-256 for all baseline/focused/release logs,
mutation evidence, semantic/deletion inventory and benchmark samples/reports.
Representative immutable receipts:

| Artifact | SHA-256 |
| --- | --- |
| closure-gates.log | `a5e760a48bb8812c0802999924ccf111bc96337196e91d857e7e0700b734e04d` |
| final-inventory.log | `362009d29cc0bc601c272f56e9fb32761b87cc3e68f0863447e375252af2e141` |
| validation-final-comparison.txt | `ffdecd5b02b7634443317b415c06d54348ad88629b7ab70044e2d2733bc1f3e8` |
| compile-comparison.txt | `22fc3127429b5e34092df94e150527ae8513b7ac1b49fa2f359f5d1f0b63d1a6` |
