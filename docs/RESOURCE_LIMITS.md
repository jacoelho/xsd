# Resource limits

Defaults bound schema compilation and document validation: per-value limits
cap one value, and aggregate pattern-work limits cap a whole document or
compilation. Set only the limits
you need to change. Zero selects the default except where stated below;
negative signed limits are errors.

## Compilation

Use `CompileWithOptions` instead of `Compile`. This helper requires the
`github.com/jacoelho/xsd` import:

```go
func compileWithLimits(schema []byte) (*xsd.Engine, error) {
	return xsd.CompileWithOptions(
		xsd.CompileOptions{MaxSchemaSourceBytes: 8 << 20, MaxSchemaSources: 32},
		xsd.Bytes("schema.xsd", schema),
	)
}
```

| Option | Default | Meaning |
| --- | ---: | --- |
| `MaxSchemaDepth` | `256` | Max nested schema XML elements. |
| `MaxSchemaAttributes` | `256` | Max attributes on one schema XML element. |
| `MaxSchemaTokenBytes` | `4 MiB` | Max bytes the schema XML reader holds for one token (names, attribute values, PI and text payload, construction scratch), measured lexically, and max character data accumulated in one element across text tokens. It also scales literal evaluation work. |
| `MaxSchemaSourceBytes` | `64 MiB` | Max bytes read from each schema source. |
| `MaxSchemaSources` | `1024` | Max explicit source descriptors, and, separately, max distinct schema documents loaded (explicit and resolver-loaded together). An unavailable include/import target loads nothing and is not counted; the document that exceeds the count is opened and closed without being read. |
| `MaxSchemaTotalBytes` | `256 MiB` | Max aggregate bytes read from schema sources. Each document is charged when first loaded; an opener source reached again is reopened and charged again; a file source is read once; an in-memory source is compared by digest without charge. |
| `MaxSchemaReferences` | `16_384` | Max include/import references processed across the schema set. |
| `MaxSchemaDependencySteps` | `1_000_000` | Max aggregate schema-graph expansion, target-context propagation, and component-dependency resolution work. Each particle appended to a content model, each attribute use and wildcard namespace admitted to a compiled set, each model record, each row, edge, `xs:all` term and name-index entry of a compiled content-model table, and each namespace that an attribute-wildcard intersection, union or restriction check looks up or merges costs one step; a compiled content model's record costs two. The compiled tables are bounded per step below. A derived type's content model holds its base particle as one particle rather than a copy of the base particles, but its attribute-use set is its own copy and is charged for every use it inherits: a base type with 300 attributes extended by 4,000 types needs about 1.2 million steps and exceeds the default. Each derived type still compiles its own content model under `MaxContentModelAnalysisSteps`, so a base type with 300 optional elements extended by 2,000 types exceeds that limit instead. |
| `MaxSchemaTargetContexts` | `4_096` | Max distinct source/effective-target-namespace contexts, including primary and chameleon-derived contexts. |
| `MaxSchemaInstantiatedNodes` | `1_000_000` | Max aggregate schema node occurrences across effective target contexts. |
| `MaxSchemaNames` | `0` | Max interned schema names, including built-ins. `0` means no explicit limit. |
| `MaxFiniteOccurs` | `0` | Max accepted finite `maxOccurs`. `0` uses the runtime `uint32` cap. |
| `MaxContentModelStates` | `16_384` | Max DFA states per compiled content model. The content model of each complex type is compiled with its nested groups; a named model group that no complex type's content model reaches is not compiled and is not subject to this limit. |
| `MaxContentModelAnalysisSteps` | `16_777_216` | Max content-model traversal, determinization, and ambiguity-analysis work. Comparing two wildcard namespace lists, for ambiguity or for a wildcard restriction, costs one step per namespace looked up. |
| `MaxSubstitutionClosureEntries` | `1_000_000` | Max aggregate transitive substitution-group relationships. |
| `MaxSimpleUnionMemberEntries` | `1_000_000` | Max aggregate flattened simple-union members. |
| `MaxSchemaPatternWork` | `536_870_912` | Max regular-expression matching work for one schema literal (enumeration, bound, fixed or default value). Same unit as `MaxInstancePatternWork`. Unsigned. |
| `MaxSchemaTotalPatternWork` | `8_589_934_592` | Max regular-expression matching work summed over every schema-literal evaluation of one compilation. A facet literal can be charged for its base-type check and again for facet compilation; a fixed/default value is charged once. Same unit. |
| `MaxSchemaPatternRetainedBytes` | `64 MiB` | Max estimated memory retained by compiled regular expressions, summed over the distinct pattern facets of one compilation. Identical pattern text is compiled and charged once. |

These limits apply to explicit sources and resolver-loaded includes/imports.
Finite `minOccurs` and `maxOccurs` values cannot exceed `4294967295`.
`MaxFiniteOccurs` can lower the finite `maxOccurs` cap;
`maxOccurs="unbounded"` is unaffected.

`MaxContentModelAnalysisSteps` also bounds the heap that compiling a content
model to its automaton uses, because each step admits a bounded amount of
automaton state. A model group referenced many times is not copied: each
position followed by the first positions of a referenced group shares that
group's set, each distinct follow set is built when a state first reaches it,
and a state waiting for its row keeps only a compact key. A sequence compiled
directly to a table shares one run of edges among the rows that take them.

In the shapes measured, the dominant term of that heap is the edges of the
determinized automaton's rows. An edge is 32 bytes on every supported platform,
and determinization charges at least three steps for each edge before it
allocates the edge, so those edges request at most 32/3, about 10.7, bytes per
analysis step. Each row's edges are one allocation, which the Go allocator
rounds up by at most about 1.249 (1025 edges: 32,800 bytes rounded to 40,960),
so the edges take at most about 13.3 allocated bytes per step: under 214 MiB
when one content model exhausts the default budget of 16,777,216 steps. This
figure is derived, not measured; the derivation is in the Go doc of `deterministicRow`
in `internal/schema/content_model_dfa.go`. It covers row edges only. A new
automaton state also keeps its key, 8 bytes per configuration, and a new
counter value its interned vector; both grow with the steps too and are
outside the figure. A model's rows, and the edges of a table compiled
directly, which holds at most one edge per state, grow with its states, which
`MaxContentModelStates` caps.

The figure bounds one content model. `MaxSchemaDependencySteps` bounds the
compiled tables summed over the content models, which a compiled schema keeps
for validation. Compilation keeps only the published form of each complex
type's root model, and charges each record of every model it compiles one step
before allocating it, including the nested models it compiles on their own and
keeps no table for: a row, an edge, an `xs:all` term and an entry of a name
index. The rows of a sequence compiled directly to a table are windows of one
run of edges, and its rows that take at least eight edges share one name
index of the run, so a sequence of optional elements holds one edge and about
one index entry per element, not one per element and row. When the run's
names are distinct, share one namespace and span at most twice as many name
IDs as there are names, the index is a dense array of run positions with one
entry per slot and two metadata entries. Other rows own their
edges and name indexes. An index is built before compilation knows that it
can be installed, so an index that is not installed is transient and never
charged. A compiled model's own record costs two steps, because publication
holds a second copy. On 64-bit platforms a row is 80 bytes, an edge and an
`xs:all` term 16 bytes, and a model record 72 bytes, held twice for its two
steps. A name-index entry is a map entry of at most 34.3 bytes, measured over
indexes of 8 to 20,000 entries with the map's growth and overhead, or a 4-byte
position in a slice. A model's rows are one allocation, its edges another and
its terms a third, and the Go allocator rounds each up by at most about 1.25
(32,848 bytes round to 40,960), so the row bounds every unit: the tables take
at most 80 × 1.25 = 100 bytes per step, under 96 MiB when they exhaust the
default budget of 1,000,000 steps. Steps the schema spends on other records
lower the figure. A schema whose content models all share one group of 16,000
required wildcards, each of its own namespace, compiles at the defaults with
29 complex types and retains about 44 MiB, about 46 bytes per step; 30 types
exceed the limit. 800 complex types that each hold a sequence of 40 optional
elements need about 167,000 steps and retain about 2.9 MiB, and the 93
document schemas of UBL 2.4 compiled together need about 54,000. A choice of
120 elements with `maxOccurs="16000"`, whose rows carry counters, compiled
with both step limits raised, retains about 60 MiB at its least limit of about
7.7 million steps, about 8 bytes per step. A sequence or choice that automaton
construction compiles after direct compilation gives up is charged for the
rows direct compilation built. While a model compiles, the heap also holds
that model's construction state, which `MaxContentModelAnalysisSteps` bounds
as above, and its transient name indexes, so the peak is the retained tables
plus one model's construction. Name indexing also keeps, for the whole
compilation, the member names of each substitution head an indexed edge
names, at most the entries `MaxSubstitutionClosureEntries` caps. The dense
dispatch indexes that publication adds to rows of at least 16 edges that own
their index have their own fixed 4 MiB cap.

The peak heap measured for whole content models, below, includes keys, counter
vectors, follow sets and scratch, and reaches about 4 bytes per step at most. In the
optional-particle sequences, the last four rows, each position is followed by
every later one:

| Content model at the default limits | Peak heap | Total allocated |
| --- | ---: | ---: |
| A choice of 256 elements doubled by 10 to 12 nested group references, a sequence of two references each | about 4 MiB | about 11 MiB |
| A choice of 2,048 elements with `maxOccurs="unbounded"` | about 2 MiB | about 4 MiB |
| A choice of 120 elements with `maxOccurs="16000"` | about 65 MiB | about 66 MiB |
| A sequence of 1,060 optional elements with `maxOccurs="2"` | about 16 MiB | about 17 MiB |
| A sequence of 2,060 optional elements, compiled directly to a table | about 3 MiB | about 5 MiB |
| A sequence of one required and 1,720 optional elements with `maxOccurs="unbounded"` | about 30 MiB | about 32 MiB |
| A sequence of one required element and 1,900 optional wildcards, each of its own namespace, with `maxOccurs="unbounded"` | about 35 MiB | about 92 MiB |

Each shape stops with `schema.limit` at its content model. Raising
`MaxContentModelAnalysisSteps` raises the bound with it.

Each pattern is compiled under fixed structural bounds that are not options:
source 1 MiB, 100,000 AST nodes, 100,000 NFA states (counted repeats expand
into states), 1,000,000 retained character ranges, and nesting depth 1,024.
Exceeding one reports `schema.limit`. These bound one compiled machine, not the
length of the values it matches. `MaxSchemaPatternRetainedBytes` is the
aggregate over a compilation.

The retained-bytes estimate of one pattern is 256 bytes of fixed overhead plus
the source length, plus 8 bytes per character range (each distinct range array
counted once, by capacity), plus, for a run-length pattern, 48 bytes per atom
and the literal text, or, for an NFA pattern, 48 bytes per state, 24 bytes per
precomputed closure, 8 bytes per closure element and 8 bytes per closure
bitset word. The default is a byte target, not a multiple of the per-pattern
caps: 64 MiB admits about 5 patterns at the per-pattern caps (about 13 MB each)
and about 10,000 Unicode-category patterns of the `[\p{L}\p{N} ]` class. The
largest aggregate observed across the W3C and project corpora under default
options is about 111 KB, so the default leaves large headroom.

## Validation

Use `ValidateWithOptions` for one call, or pass the same options to `NewSession`.
This helper requires `io` and `github.com/jacoelho/xsd`:

```go
func validateWithLimits(engine *xsd.Engine, doc io.Reader) error {
	return engine.ValidateWithOptions(doc, xsd.ValidateOptions{
		MaxErrors:        1,
		MaxInstanceBytes: 8 << 20,
	})
}
```

| Option | Default | Meaning |
| --- | ---: | --- |
| `MaxErrors` | `100` | Max collected recoverable validation errors. |
| `MaxIdentityScopes` | `10_000` | Max active identity-constraint scopes. |
| `MaxIdentityEntries` | `100_000` | Independent max for stored ID, IDREF, key, unique, and keyref entries, pending identity-selector matches, and pending identity-field values. |
| `MaxIdentityTupleBytes` | `4 KiB` | Max byte length of one stored identity key. |
| `MaxInstanceDepth` | `256` | Max nested XML elements. |
| `MaxInstanceAttributes` | `4_096` | Max attributes on one XML element. |
| `MaxInstanceTextBytes` | `4 MiB` | Max retained character data bytes. |
| `MaxInstanceTokenBytes` | `4 MiB` | Max parser-owned bytes for one XML token, including retained payload and active construction scratch. |
| `MaxInstanceBytes` | `64 MiB` | Max aggregate raw XML bytes read, including a UTF-8 BOM and XML declaration. |
| `MaxInstanceValueWork` | `4_194_502_132_335` | Max cumulative lexical work per simple-value evaluation. Independent of schema compilation limits. |
| `MaxInstancePatternWork` | `536_870_912` | Max regular-expression matching work per simple value, across all pattern facets, list items and union members. |
| `MaxInstanceTotalPatternWork` | `8_589_934_592` | Max regular-expression matching work summed over all values of one document. |

Each diagnostic path is at most 4096 bytes and each name in it at most 128
bytes before escaping, so retained diagnostic text is bounded by `MaxErrors`.

Each value-evaluation visit charges its lexical byte length plus one. List items,
union attempts, and a raw-byte attempt followed by typed fallback share the
containing value's budget. Each new value starts a fresh budget. URI items in
`xsi:schemaLocation` hints are separate typed evaluations. The work limit also
applies to schema defaults or fixed values that require revalidation; already
validated defaults keep their existing path. Zero selects the default; positive
values set a finite work ceiling. Pattern matching is charged to its own budget,
described below, not to this one.

Enumeration membership is not charged against this budget.

### Pattern matching cost

Each value has one per-value pattern budget (`MaxInstancePatternWork`, or
`MaxSchemaPatternWork` for schema literals). It is shared by every pattern
group, list item and union attempt of that value, and a new value starts a
fresh budget. It caps one value. A second, aggregate budget caps the sum over
every value of one document (`MaxInstanceTotalPatternWork`) or of one
compilation (`MaxSchemaTotalPatternWork`); a new document or compilation starts
a fresh total. A match costs:

- one unit per input byte, charged before the input is decoded;
- literal patterns: nothing further, so input bytes only;
- linear patterns (concatenations of repeated character sets): `4 * (runes + 1) + 1`
  per atom, literal atoms and exact-count atoms included. Example: `x[a-z]+` on
  `x` followed by 1000 `a` costs 1001 + 2 * (4 * 1002 + 1) = 9019;
- NFA patterns: one unit per set-state test and per split or jump followed
  during epsilon closure, so work is at most 72 * (runes + 1) * compiled states.

Exhaustion reports `validation.limit` naming `MaxInstancePatternWork` (or
`schema.limit` naming `MaxSchemaPatternWork`). When the remaining aggregate
budget is strictly smaller than the remaining per-value budget, the diagnostic
names `MaxInstanceTotalPatternWork` (or `MaxSchemaTotalPatternWork`); on a tie
it names the per-value option. A value up to `MaxInstanceTextBytes` matches
`[a-z]+` within the default.

The aggregate default is derived, not chosen: the per-value default times the
number of maximum-size text values a maximum-size document can hold,
`2^29 * (64 MiB / 4 MiB) = 2^33`. A maximum-size document made entirely of
maximum-size values of the shape the per-value default admits therefore stays
admitted. At the measured worst case of about 1 s per value, the total bounds
pattern CPU for one document at about 16 s. The schema default is the
per-literal default times 16, the same ratio.

Measured under default options on one machine (macOS, serialized), 4 MiB of
text:

| Pattern | Work | Time |
| --- | ---: | ---: |
| `[a-z]+` | 20,971,525 | 17 ms |
| `(a\|b)*c` | 33,554,435 | 51 ms |
| `(a\|aa){1,300}(a{1,100}){1,300}b` (61,500 states, non-matching) | exceeds the default | about 1.2 s until `MaxInstancePatternWork` is reached |

At the earlier candidate of `1 << 30` the last case took 2.3 s before reaching
the per-value limit, so the per-value default is `1 << 29`, the largest power
of two that keeps one value near 1 s while admitting both common cases with a
wide margin.

Reaching `MaxErrors` stops semantic assessment, but XML is still read until the
end or a fatal error. A later fatal error takes precedence.
