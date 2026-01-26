# XSD Refactor Single Source of Truth (v5)

Status: normative refactor specification + implementation task list
Compatibility: backward compatibility is explicitly not required
Primary priority: maximum validation throughput for “many documents against one schema” workloads

---

## 0. North Star (starting point)

### 0.1 Workload assumption

The dominant workload is **validating many instance documents against one compiled schema**.

Implication: the architecture must optimize for:

1. **Compile once**, reuse indefinitely.
2. **Per-document execution** must be cheap: minimal allocations, minimal hashing, maximal cache locality.
3. Concurrency comes from **many independent validations in parallel**, not from sharing a single reader.

### 0.2 Target end-state runtime architecture

**Engine (shared, immutable)**

- Owns the compiled schema runtime model.
- Owns all compile-time caches and indices.
- Safe for concurrent use by many goroutines.

**Session (per document, reusable)**

- Owns all mutable per-document state:
  - element stack frames
  - in-scope namespace stack
  - content-model state machines
  - identity-constraint state
  - reusable scratch buffers (text normalization, parsing, error building)
- Intentionally **not thread-safe**. It is **goroutine-confined**.

**Borrowed stream (per session)**

- `xmlstream.Reader` is not thread-safe.
- A session never shares a reader; it may reuse a reader sequentially across documents.

### 0.3 Public API shape (new)

```go
package xsd

type Engine struct {
    rt   *runtime.Schema   // immutable, shareable
    pool sync.Pool         // *Session
}

func CompileSchema(r io.Reader, opts ...CompileOption) (*Engine, error)
func CompileFS(fsys fs.FS, root string, opts ...CompileOption) (*Engine, error)

func (e *Engine) Validate(r io.Reader, opts ...ValidateOption) error
func (e *Engine) NewSession() *Session
```

`Engine.Validate` acquires a session from the pool, validates, resets, returns it to the pool.

---

## 1. Primary failure modes if issues remain

These are the non-negotiable outcomes to avoid:

1. **Incorrect validation semantics**
   Especially for:
   - QName parsing and resolution (schema-time and instance-time)
   - wildcard / anyAttribute matching and processContents behavior
   - xsi:nil / default / fixed / simpleContent handling
   - xsi:type retargeting
   - substitution-group matching vs validation target

2. **Non-deterministic builds**
   The same schema set must compile to byte-identical runtime tables (unless build options change).

3. **Runtime gaps for common schema constructs**
   Particularly:
   - local element/attribute declarations
   - anonymous types
   - simpleContent and complexContent derivations
   - global attributes and attribute refs

---

## 2. Hard invariants and performance contract

### 2.1 Correctness invariants

The following invariants MUST hold:

- Compiled runtime schema is **immutable**.
- No validation correctness depends on goroutine scheduling or map iteration order.
- Conformance test suite (W3C + curated real schemas) passes.

### 2.2 Throughput invariants

Success path targets (valid documents, no errors):

- No allocations per element is the ideal; “near zero allocations” is acceptable only for:
  - identity constraints table growth
  - rare schema constructs that inherently require per-value storage

Hot path bans:

- `string([]byte)` conversions in the normal success path.
- `map` lookups by string/QName in the normal success path.
- interface-based polymorphism in per-element loops (use `switch` + numeric IDs).

### 2.3 Concurrency invariants

- Engine is safe for concurrent use with **zero locks in the hot path**.
- Sessions are goroutine-confined; no attempt to make a single session thread-safe.
- `sync.Pool` is permitted for sessions; if contention becomes visible, shard pools (per-P or per-engine shards).

---

## 3. Model and ID system

All core schema components are assigned dense IDs for array indexing.

### 3.1 ID types

```go
type SymbolID    uint32 // QName intern ID (namespace + local)
type NamespaceID uint32 // Namespace URI intern ID (namespace only)

type TypeID      uint32
type ElemID      uint32
type AttrID      uint32
type ModelID     uint32
type WildcardID  uint32
type ICID        uint32 // identity-constraint ID
type PathID      uint32 // compiled XPath program ID
type ValidatorID uint32
```

ID 0 is always reserved as “invalid / not found”.

### 3.2 Symbols vs namespaces

**SymbolID** identifies an expanded name (namespace URI + local name).
**NamespaceID** identifies a namespace URI (no local part).

Why both exist:

- Most hot lookups (elements/types/attrs) use SymbolID.
- Wildcards and anyAttribute must match by namespace even for **unknown names**. Unknown names often have `SymbolID == 0`, but their namespace URI is still known from the instance document. Therefore wildcard matching must not depend on SymbolID alone.

---

## 4. Loader and resolution semantics (includes/imports/base path)

This section is normative and eliminates “same as v2” ambiguity.

### 4.1 Compile options for loading

```go
type Resolver interface {
    // Resolve returns the schema document bytes and a canonical SystemID for caching/dedup.
    // schemaLocation may be empty (e.g., <import namespace="..."/>).
    Resolve(req ResolveRequest) (doc io.ReadCloser, systemID string, err error)
}

type ResolveRequest struct {
    BaseSystemID   string // canonical base ID of the referencing schema
    SchemaLocation string // raw schemaLocation attribute (may be empty for import)
    ImportNS       []byte // namespace attribute on <import> (may be empty/absent)
    Kind           ResolveKind // Include or Import
}

type ResolveKind uint8
const (
    ResolveInclude ResolveKind = iota
    ResolveImport
)

type CompileOption interface{ apply(*compileOptions) }

func WithResolver(r Resolver) CompileOption
func WithFS(fsys fs.FS) CompileOption                  // default resolver for relative paths
func WithBaseSystemID(base string) CompileOption       // required for CompileSchema(io.Reader) if includes/imports are expected
func WithAllowMissingImportLocations(b bool) CompileOption
func WithCompileLimits(l CompileLimits) CompileOption
```

Defaults:

- `CompileFS(fsys, root)` uses FS resolver and `BaseSystemID = root`.
- `CompileSchema(io.Reader)` uses **no resolver** unless provided:
  - includes/imports with schemaLocation => error unless resolver set
  - import without schemaLocation => allowed only if `WithAllowMissingImportLocations(true)` or resolver supports namespace-based resolution
- `CompileLimits` defaults: `MaxDFAStates=4096`, `MaxOccursLimit=1_000_000`.

### 4.2 Path validation and canonicalization (FS resolver)

All schemaLocation paths passed to FS resolver must pass strict validation:

- Reject absolute paths (start with `/`).
- Reject any segment equal to `.` or `..` (prevents traversal even if later cleaned).
- Reject empty paths.
- Reject backslash `\` (force `/` separators).
- Canonical path is the exact join of validated segments with `/`. No `path.Clean` is applied because it can erase traversal markers.

Rationale: prior “clean then check” is internally inconsistent because cleaning can remove traversal markers.

### 4.3 Include constraints and chameleon include

XSD include constraints (normative):

- `<include schemaLocation="...">` requires that the included schema’s targetNamespace matches the including schema’s targetNamespace, **or** both are absent, **or** the included schema has no targetNamespace and the including schema has a targetNamespace (chameleon include).

Chameleon include handling:

- If included schema has **absent targetNamespace** and including schema has a non-absent targetNamespace, then the included schema is treated as if that targetNamespace were present “anywhere the absent target namespace name would have appeared”.
- Implementation rule (deterministic):
  - Each loaded schema document gets an **EffectiveTargetNamespace (ETN)**:
    - If declared targetNamespace present: ETN = declared
    - If absent and included by schema with ETN=T: ETN = T (chameleon)
    - If absent and included by schema with ETN absent: ETN absent
  - The caching key for a schema document is `(systemID, ETN)` because the same physical document may be included as a chameleon into multiple target namespaces.

### 4.4 Import semantics and schemaLocation optionality

- `<import>` may omit both `namespace` and `schemaLocation`.
- `<import namespace="..."/>` with no schemaLocation is allowed.

Loader behavior (normative, deterministic):

1. If schemaLocation is present (non-empty):
   - if resolver is absent: error `LOADER_NO_RESOLVER`
   - else call resolver; if not found => error unless `WithAllowMissingImportLocations(true)` and error is `fs.ErrNotExist`
2. If schemaLocation is absent/empty:
   - if resolver is present: call resolver anyway with empty SchemaLocation and ImportNS; resolver may succeed via catalog/namespace mapping
   - if resolver absent:
     - if `WithAllowMissingImportLocations(true)`: record “imported namespace present, no documents loaded”
     - else: error `LOADER_IMPORT_MISSING_LOCATION`

Resolution later:
- If a QName refers to an imported namespace for which no documents were loaded, resolution fails with `RESOLVE_UNDEFINED_*` at the point of use. This is deterministic.

### 4.5 Discovery order and determinism

Determinism depends on preserving include/import order exactly as written.

Rule:

- Loader MUST scan schema directives (`include`, `import`, `redefine`) among the direct children of `<schema>` in document order and enqueue them in that exact order.
- If `redefine` is encountered, error `LOADER_REDEFINE_UNSUPPORTED` (no attempt to load its target).

Dedup/caching rule:

- A schema document instance `(systemID, ETN)` is parsed at most once. Multiple references enqueue the same key but processing is skipped after the first successful parse.
- This avoids infinite loops for cyclic include/import graphs while preserving deterministic “first discovery” ordering.

### 4.6 Cycle handling (loader-level)

Loader must detect and safely handle cycles:

- Maintain `state[key] = {Unseen, Loading, Loaded}`.
- When a dependency is encountered:
  - if Unseen: mark Loading, parse header, enqueue deps, then parse full doc, mark Loaded
  - if Loading: this is a cycle edge; record it but do not recurse (no re-parse)
  - if Loaded: ignore

Cyclic include graphs are allowed; they simply merge into the same schema set, subject to component uniqueness constraints.

---

## 5. QName handling (schema-time and instance-time)

### 5.1 Schema-time QName interpretation (parsing schema documents)

Schema-time QName interpretation uses **in-scope namespaces including default namespace** when expanding a QName lexical value.

Schema-time QName resolution imposes additional constraints:

- If the namespace name is not absent, it must be either:
  - the schema’s own targetNamespace, or
  - a namespace named in an `<import>` in the schema.
- If the namespace name is absent, then:
  - the schema must have no targetNamespace, or
  - the schema must have an `<import>` with no namespace.

Implementation rules:

- While parsing schema documents, maintain a namespace context stack exactly like the instance validator.
- Every QName-valued schema attribute is parsed using the namespace context **at that element**.

This replaces the ambiguous “resolve using schema namespace mappings” phrasing.

### 5.2 Instance-time QName interpretation (validation)

Instance-time QName values (e.g., `xsi:type`, values of type `xs:QName`) must be expanded using the **in-scope namespaces of the instance element** (prefix→URI), including default namespace if applicable.

Validator API impact:

- QName parsing cannot be a pure function of bytes; it must accept a namespace resolver.

---

## 6. Identity constraints XPath subset (fully specified)

Restricted XPath subset is exactly the XSD 1.0 grammar:

Selector expressions:

- Union of Paths: `Path ('|' Path)*`
- Path: optional `.//` prefix, then Steps separated by `/`
- Step is `.` or NameTest
- NameTest is `QName | '*' | NCName ':' '*'`

Field expressions:

- Same as selector, except final step may be attribute: `... (Step | '@' NameTest)`

Whitespace may appear between tokens and is ignored; tokenization chooses the longest possible token.

### 6.1 Compilation to bytecode

We compile each selector/field into a `PathProgram`:

```go
type Op uint8
const (
    OpRootSelf Op = iota // leading '.' in Path (root self)
    OpSelf               // '.' step within a path
    OpDescend            // leading './/' (descendant-or-self then child steps)
    OpChildName          // child::QName
    OpChildAny           // child::*
    OpChildNSAny         // child::NCName:*
    OpAttrName           // @QName (field final step only)
    OpAttrAny            // @*
    OpAttrNSAny          // @NCName:* (allowed by NameTest)
    OpUnionSplit         // internal: marks branch boundaries for '|' (or represent as multiple programs)
)
type PathProgram struct {
    Ops []PathOp // flattened; unions are compiled as multiple PathIDs
}
type PathOp struct {
    Op     Op
    Sym    SymbolID    // for QName
    NS     NamespaceID // for prefix:* and for QName fast ns compare
}
```

Union representation: compile each `Path` alternative into its own PathID. A selector thus becomes `[]PathID`.

Compilation note: a `.` step within a path emits `OpSelf` and is a no-op in matching (it only preserves the current node).

### 6.2 Runtime evaluation model (streaming)

Per active identity constraint, the session maintains:

- For each selector path alternative:
  - a set of “currently matched depth positions”
- For each field path:
  - an evaluator that activates when selector matches, then collects value at the appropriate descendant/child/attribute node

Evaluation is depth-driven:

- On StartElement:
  - update selector evaluators
  - if a selector matches, create a new “row” in the constraint’s node-table-in-progress for this selected node
- On attributes:
  - evaluate field programs ending in `@...`
- On text end (simple content):
  - evaluate field programs ending in child step selecting the element itself (field path points to a child element; value is collected when that field element closes)

Correctness: we do not support axes other than child/attribute, and do not support predicates (the XSD subset does not include them in this edition).

---

## 7. Runtime model: fully defined structs (no undefined placeholders)

This section resolves the “undefined runtime types” criticism by defining every referenced entity.

### 7.1 Schema runtime root

```go
package runtime

type Schema struct {
    // Intern pools (immutable)
    Symbols    SymbolsTable
    Namespaces NamespaceTable

    // Global indices (fast path): arrays indexed by SymbolID.
    // Length MUST be Symbols.Count()+1; entry 0 unused; missing maps to 0.
    GlobalTypes      []TypeID
    GlobalElements   []ElemID
    GlobalAttributes []AttrID

    // Component tables (immutable)
    Types        []Type
    Ancestors    TypeAncestors
    ComplexTypes []ComplexType
    Elements     []Element
    Attributes   []Attribute
    AttrIndex    ComplexAttrIndex

    // Validators and value tables (immutable)
    Validators ValidatorsBundle
    Facets     []FacetInstr // global facet bytecode for all validators
    Patterns   []Pattern    // compiled regex patterns referenced by facets
    Enums      EnumTable
    Values     ValueBlob

    // Content models (immutable)
    Models    ModelsBundle
    Wildcards []WildcardRule
    WildcardNS []NamespaceID // flattened list referenced by WildcardRule.NS.Off/Len

    // Identity constraints (immutable)
    ICs        []IdentityConstraint
    ElemICs    []ICID     // flattened list referenced by Element.ICOff/ICLen
    ICSelectors []PathID  // flattened list referenced by IdentityConstraint.SelectorOff/Len
    ICFields    []PathID  // flattened list referenced by IdentityConstraint.FieldOff/Len
    Paths      []PathProgram

    // Precomputed SymbolIDs and built-in TypeIDs used in hot paths
    Predef  PredefinedSymbols
    PredefNS PredefinedNamespaces
    Builtin BuiltinIDs

    // Policies
    RootPolicy RootPolicy

    // Determinism/versioning
    BuildHash uint64
}

type BuiltinIDs struct {
    AnyType       TypeID // built-in xs:anyType
    AnySimpleType TypeID // built-in xs:anySimpleType
}

type PredefinedSymbols struct {
    XsiType                    SymbolID
    XsiNil                     SymbolID
    XsiSchemaLocation          SymbolID
    XsiNoNamespaceSchemaLocation SymbolID

    XmlLang  SymbolID
    XmlSpace SymbolID
}

type PredefinedNamespaces struct {
    Xsi NamespaceID
    Xml NamespaceID
    Empty NamespaceID // absent namespace
}

type RootPolicy uint8
const (
    RootStrict RootPolicy = iota // root must be a global element
    RootAny                      // deprecated alias of RootStrict; kept only for API compatibility
)
```

RootAny semantics:

- RootAny is a deprecated alias of RootStrict. Behavior is identical.
- Rationale: “accept any root” without validation is unsafe and conflicts with correctness.

### 7.2 Intern tables

#### 7.2.1 Namespace table

```go
type NamespaceTable struct {
    Blob []byte       // concatenated namespace URI bytes
    Off  []uint32     // offset per NamespaceID (1-based)
    Len  []uint32     // length per NamespaceID
    // open-address hash index: maps (hash, bytes) -> NamespaceID
    Index NamespaceIndex
}

type NamespaceIndex struct {
    // power-of-two sized arrays
    Hash []uint64
    ID   []NamespaceID // 0 empty
}
```

NamespaceID 0 represents “unknown namespace at runtime lookup”.
Empty namespace (“absent”) is a real namespace; it must have a real NamespaceID (typically 1) and is stored in `PredefNS.Empty`.

#### 7.2.2 Symbol table (QName)

```go
type SymbolsTable struct {
    // Each symbol references a namespace and a local-name blob range.
    NS     []NamespaceID // per SymbolID
    LocalOff []uint32
    LocalLen []uint32
    LocalBlob []byte

    Index SymbolsIndex // maps (nsID, localBytes) -> SymbolID
}

type SymbolsIndex struct {
    Hash []uint64
    ID   []SymbolID
}
```

SymbolID 0 is invalid / not found.

### 7.3 Types and validators

#### 7.3.1 Type table

```go
type TypeKind uint8
const (
    TypeBuiltin TypeKind = iota
    TypeSimple
    TypeComplex
)

type Type struct {
    Kind TypeKind
    Name SymbolID // 0 for anonymous types

    Flags TypeFlags

    // Base type (for derivation checks)
    Base       TypeID
    Derivation DerivationMethod // Extension|Restriction|List|Union

    // final/block on the TYPE itself (used by xsi:type checks)
    Final DerivationMethod
    Block DerivationMethod

    // Precomputed ancestor chain for IsDerivedFrom fast checks
    AncOff     uint32
    AncLen     uint32
    AncMaskOff uint32 // aligned with ancestors: cumulative derivation mask to that ancestor

    // For simple types
    Validator ValidatorID // 0 for complex types

    // For complex types
    Complex ComplexTypeRef // 0 if not complex
}

type TypeFlags uint32
const (
    TypeAbstract TypeFlags = 1 << iota
)
type DerivationMethod uint8
const (
    DerNone DerivationMethod = 0
    DerExtension DerivationMethod = 1 << 0
    DerRestriction DerivationMethod = 1 << 1
    DerList DerivationMethod = 1 << 2
    DerUnion DerivationMethod = 1 << 3
)
```

Runtime tables:

```go
type TypeAncestors struct {
    IDs   []TypeID
    Masks []DerivationMethod // cumulative masks aligned with IDs
}
```

`Schema.Ancestors` stores these arrays; `Type.AncOff/AncLen/AncMaskOff` index into them.

`IsDerivedFrom(child, base) -> (ok bool, mask DerivationMethod)` is implemented by scanning the child’s ancestor IDs and returning the aligned mask. This resolves the “method mask missing” issue.

#### 7.3.2 Validators bundle (all kinds fully represented)

ValidatorsBundle must include storage for all validator kinds that runtime uses:

```go
type ValidatorKind uint8
const (
    VString ValidatorKind = iota
    VBoolean
    VDecimal
    VInteger
    VFloat
    VDouble
    VDuration
    VDateTime
    VTime
    VDate
    VGYearMonth
    VGYear
    VGMonthDay
    VGDay
    VGMonth
    VAnyURI
    VQName
    VNotation
    VHexBinary
    VBase64Binary
    // plus any derived built-ins if represented separately
    VList
    VUnion
)

type ValidatorsBundle struct {
    // One “record array per kind” avoids interfaces and enables tight packing.
    String   []StringValidator
    Boolean  []BooleanValidator
    Decimal  []DecimalValidator
    Integer  []IntegerValidator
    Float    []FloatValidator
    Double   []DoubleValidator
    Duration []DurationValidator
    DateTime []DateTimeValidator
    Time     []TimeValidator
    Date     []DateValidator
    GYearMonth []GYearMonthValidator
    GYear     []GYearValidator
    GMonthDay []GMonthDayValidator
    GDay      []GDayValidator
    GMonth    []GMonthValidator
    AnyURI   []AnyURIValidator
    QName    []QNameValidator
    Notation []NotationValidator
    HexBinary []HexBinaryValidator
    Base64Binary []Base64BinaryValidator
    // ...
    List     []ListValidator
    Union    []UnionValidator

    // Indirection table: ValidatorID -> (Kind, Index)
    Meta []ValidatorMeta
}

type ValidatorMeta struct {
    Kind  ValidatorKind
    Index uint32 // index into the corresponding slice
    WhiteSpace WhitespaceMode
    Facets FacetProgramRef
}

type WhitespaceMode uint8
const (
    WS_Preserve WhitespaceMode = iota
    WS_Replace
    WS_Collapse
)
```

FacetProgram is a compact, precompiled representation:

```go
type FacetProgramRef struct {
    Off uint32
    Len uint32
}
type FacetOp uint8
const (
    FPattern FacetOp = iota
    FEnum
    FMinInclusive
    FMaxInclusive
    FMinExclusive
    FMaxExclusive
    FMinLength
    FMaxLength
    FLength
    FTotalDigits
    FFractionDigits
)
type FacetInstr struct {
    Op FacetOp
    Arg0 uint32
    Arg1 uint32
}
type FacetProgram struct {
    Code []FacetInstr // global
}
```

This resolves “ValidatorsBundle omits kinds” and “unimplementable types”.

#### 7.3.3 QName value canonicalization

Canonical representation for QName-typed values:

- `canon = uriBytes + 0x00 + localBytes`
- Empty uriBytes represents absent namespace.

Equality:
- byte-equality of canonical bytes.

Hash:
- deterministic 64-bit hash of canonical bytes (fixed seed).

This eliminates nondeterminism from multiple encodings.

### 7.4 Elements and attributes

#### 7.4.1 Element table

```go
type Element struct {
    Name SymbolID

    Type TypeID // declared type
    SubstHead ElemID // 0 if not in substitution group

    Default ValueRef // Present=true when specified
    Fixed   ValueRef // Present=true when specified

    Flags ElemFlags
    Block ElemBlock        // block on xsi:type / substitution
    Final DerivationMethod // final on derivation (for substitution group membership)

    // identity constraints scoped here (keys, uniques, keyrefs)
    // indexes into Schema.ElemICs
    ICOff uint32
    ICLen uint32
}

type ElemFlags uint32
const (
    ElemNillable ElemFlags = 1 << iota
    ElemAbstract
    // add bits as needed; all bits must be documented here
)

type ElemBlock uint8
const (
    ElemBlockSubstitution ElemBlock = 1 << iota
    ElemBlockExtension
    ElemBlockRestriction
)

type ValueRef struct {
    Off uint32
    Len uint32
    Hash uint64
    Present bool // distinguishes empty string from absent value
    // ValidatorID is not stored here; it comes from element/attr type
}
```

#### 7.4.2 Attribute table (global attributes)

```go
type Attribute struct {
    Name SymbolID
    Validator ValidatorID // attribute types are simple
    Default ValueRef // Present=true when specified
    Fixed   ValueRef // Present=true when specified
}
```

Global attribute index exists for:

- attribute refs (`ref=...`)
- anyAttribute strict/lax global lookup

#### 7.4.3 Attribute use and complex type attribute index

```go
type AttrUse struct {
    Name SymbolID
    Validator ValidatorID
    Use AttrUseKind
    Default ValueRef // if Present, overrides decl default
    Fixed   ValueRef // if Present, overrides decl fixed
}
type AttrUseKind uint8
const (
    AttrOptional AttrUseKind = iota
    AttrRequired
    AttrProhibited
)

type AttrIndexRef struct {
    Off uint32
    Len uint32 // widened from uint16 to avoid silent caps
    Mode AttrIndexMode
    HashTable uint32 // index into ComplexAttrIndex.HashTables when Mode==AttrIndexHash
}
type AttrIndexMode uint8
const (
    AttrIndexSmallLinear AttrIndexMode = iota
    AttrIndexSortedBinary
    AttrIndexHash
)

type ComplexAttrIndex struct {
    Uses []AttrUse // global pool
    // For hash mode: open-address table per complex type is stored in a global blob with ref
    HashTables []AttrHashTable
}
type AttrHashTable struct {
    Hash []uint64
    Slot []uint32 // index+1 into Uses slice; 0 empty
}
```

Lookup algorithm is fully defined:

- If Len <= 8: linear scan
- Else if Len <= 64: sorted by Name SymbolID, binary search
- Else: hash table

Thresholds are explicit and can be tuned later.
Hash table slots store `useIndex+1` so zero can represent “empty”.

### 7.5 Wildcards

Wildcard rules are explicitly represented and used by both elements and attributes.

```go
type ProcessContents uint8
const (
    PCStrict ProcessContents = iota
    PCLax
    PCSkip
)

type NSConstraintKind uint8
const (
    NSAny NSConstraintKind = iota
    NSOther
    NSEnumeration
)

type NSConstraint struct {
    Kind NSConstraintKind
    HasTarget bool // for enumeration: includes ##targetNamespace
    HasLocal  bool // for enumeration: includes ##local
    Off uint32      // NamespaceID list
    Len uint32
}

type WildcardRule struct {
    NS NSConstraint
    PC ProcessContents
    TargetNS NamespaceID // targetNamespace of the declaring schema (for ##targetNamespace)
}
```

`NSConstraint.Off/Len` indexes into `Schema.WildcardNS`, a flattened NamespaceID list.

Matching is **namespace-based** and does not require SymbolID. `##targetNamespace` is evaluated against `WildcardRule.TargetNS` (the declaring schema’s target namespace), not a schema-set global:

```go
func (w WildcardRule) Accepts(nsBytes []byte, nsID NamespaceID, nsTable *NamespaceTable) bool
```

Implementation note: if `nsID==0`, `Accepts` must fall back to byte-compare against namespace URIs obtained from `nsTable` (including `TargetNS`).

This resolves the “unknown name wildcard match failure” issue.

### 7.6 Content models and models bundle

#### 7.6.1 Model references

```go
type ContentKind uint8
const (
    ContentEmpty ContentKind = iota
    ContentSimple // simpleContent
    ContentMixed
    ContentElementOnly
    ContentAll
)

type ComplexTypeRef struct {
    ID uint32 // index into ComplexTypes slice; 0 invalid
}

type ComplexType struct {
    Content ContentKind

    // Attributes
    Attrs AttrIndexRef
    AnyAttr WildcardID // 0 if none

    // For simpleContent: validator for text
    TextValidator ValidatorID // only if ContentSimple
    TextFixed ValueRef        // fixed value for simple content if any (Present=true)
    TextDefault ValueRef      // default if any (Present=true)

    // For element/mixed/all:
    Model ModelRef // 0 if empty
    Mixed bool
}

type ModelRef struct {
    Kind ModelKind
    ID   uint32 // index into the model-kind slice
}
type ModelKind uint8
const (
    ModelNone ModelKind = iota
    ModelDFA
    ModelNFA // optional fallback, but still defined
    ModelAll
)
```

#### 7.6.2 Models bundle (fully specified)

```go
type ModelsBundle struct {
    DFA []DFAModel
    NFA []NFAModel
    All []AllModel
}

type DFAModel struct {
    Start uint32
    States []DFAState
    // transition storage is global and referenced by offsets
    Transitions []DFATransition
    Wildcards []DFAWildcardEdge
}

type DFAState struct {
    Accept bool
    TransOff uint32
    TransLen uint32
    WildOff uint32
    WildLen uint32
}

type DFATransition struct {
    Sym SymbolID
    Next uint32
    // Declared element to validate (member element for substitution groups)
    Elem ElemID
}

type DFAWildcardEdge struct {
    Rule WildcardID
    Next uint32
}
```

Acceptance check is explicit:

- On EndElement, the current DFA state MUST be `Accept==true` else error `cvc-complex-type.2.4.b` or equivalent.

All-model representation:

```go
type AllModel struct {
    MinOccurs uint32
    Mixed bool
    Members []AllMember
}
type AllMember struct {
    Elem ElemID
    Optional bool
    AllowsSubst bool
}
```

This addresses “AllModel has no storage” and “end-of-element acceptance unspecified”.

### 7.7 Identity constraints runtime types (fully specified)

```go
type IdentityConstraint struct {
    Name SymbolID
    Category ICCategory
    SelectorOff uint32 // []PathID
    SelectorLen uint32
    FieldOff uint32    // []PathID
    FieldLen uint32
    Referenced ICID // for keyref; 0 otherwise
}
type ICCategory uint8
const (
    ICUnique ICCategory = iota
    ICKey
    ICKeyRef
)

type PathProgram struct {
    Ops []PathOp
}
```

---

## 8. Validation semantics (streaming session)

### 8.1 Session structure

```go
type NameID uint32 // per-reader interned name ID (scope is a single document)
type nameEntry struct {
    Sym SymbolID
    NS  NamespaceID
    LocalOff uint32
    LocalLen uint32
    NSOff    uint32
    NSLen    uint32
}

type Session struct {
    rt *runtime.Schema

    // stacks
    elemStack []elemFrame
    nsStack   []nsFrame

    // scratch buffers reused across documents
    textBuf []byte
    normBuf []byte
    errBuf  []byte

    // caches for current document
    nameMap []nameEntry // NameID->(SymbolID, NamespaceID, ns/local bytes refs)
    // identity state
    icState identityState

    // xmlstream reader reused sequentially (optional)
    reader xmlstream.Reader
}
```

Reset rules per document (critical):

- `nameMap` MUST be cleared per document; NameID is per-reader and reusing the map across documents can corrupt validation.
- all stacks reset to len=0
- identity state reset
- scratch buffers retained (capacity reused)

### 8.2 StartDocument and root policy

Algorithm:

1. Read first StartElement.
2. Resolve its `(nsURI, local)` to:
   - NamespaceID via NamespaceTable.Index (or 0 if unknown)
   - SymbolID via SymbolsTable.Index (or 0 if not a declared name)
3. If root policy is RootStrict:
   - lookup global element by SymbolID, else error `VALIDATE_ROOT_NOT_DECLARED`
4. If root policy is RootAny:
   - same behavior (deprecated alias of RootStrict)

### 8.3 StartElement algorithm (fully specified, including xsi:type and wildcards)

For each element StartElement event:

Inputs from xmlstream:

- nsBytes []byte (namespace URI)
- localBytes []byte
- symID SymbolID (0 if not in schema symbol table)
- nsID NamespaceID (0 if unknown)
- attributes list (including xmlns declarations already applied to ns context)

Steps:

1. Determine expected particle match from parent frame’s content model:
   - `match = parent.model.Feed(symID, nsBytes, nsID)`
   - `match` yields one of:
     - (a) declared ElemID (exact transition)
     - (b) wildcard edge with WildcardID
     - (c) missing/invalid
2. If match is exact ElemID: `declElem = rt.Elements[ElemID]`
3. If match is wildcard:
   - Evaluate wildcard `rule = rt.Wildcards[WildcardID]`
   - If `rule.Accepts(nsBytes, nsID, &rt.Namespaces)` is false: error `VALIDATE_WILDCARD_NO_MATCH`
   - Else apply processContents:
     - Strict: try `GlobalElements.Lookup(symID)`; if not found => error `VALIDATE_WILDCARD_ELEM_STRICT_UNRESOLVED`
     - Lax: try lookup; if not found => treat as “skip element validation”, but still must maintain well-formedness and depth
     - Skip: always skip element validation
   - If lookup succeeds => `declElem` is the looked-up global element.
4. Abstract element check:
   - If `declElem.Flags&ElemAbstract != 0`: error `VALIDATE_ELEMENT_ABSTRACT`.
5. Substitution groups:
   - Content-model transitions MUST carry the **member ElemID** (not the head) for member element names.
   - Therefore, if a transition is taken, `declElem` is already the member ElemID.
   - This resolves the “validated against head type” failure mode.
6. Namespace/context setup:
   - Push namespace declarations into nsStack.
7. Attribute pre-scan:
   - Identify `xsi:type` and `xsi:nil` by comparing attribute SymbolID to precompiled `Predef.XsiType` and `Predef.XsiNil`.
   - Parse xsi:type first (if present), because it changes validation target type.
8. Determine actual type:
   - Start with declared type: `actualType = declElem.Type`
   - If xsi:type present:
     - parse QName lexical using **instance ns context**
     - resolve QName to `TypeID` via `GlobalTypes`
     - enforce derivation: `resolvedType` must be validly derived from `declElem.Type` and must not be blocked by `declElem.Block` (extension/restriction) and must not violate `resolvedType.Final`.
     - if ok: `actualType = resolvedType`
   - Retargeting requirement: attribute and content validation MUST use `actualType`, not `declElem.Type`.
9. xsi:nil handling:
   - If xsi:nil present and true:
     - require `declElem.Flags&ElemNillable != 0`
     - if `declElem.Fixed.Present`: error `VALIDATE_NILLED_HAS_FIXED`
     - record frame as `nilled=true`
     - enforce empty content: element must have **no character or element children**.
     - still validate attributes against `actualType` attribute rules.
10. Validate attributes:
   - If `actualType.Kind != TypeComplex` (simple or builtin), only attributes in the XSI namespace (`rt.PredefNS.Xsi`) are permitted.
   - Use `actualType`’s `ComplexType.Attrs` and `AnyAttr` wildcard.
   - Duplicate attribute rule: duplicates are determined by expanded name (namespace URI + local name), not by raw prefix.
   - If a declared attribute use is `AttrProhibited` and the attribute is present: error.
   - anyAttribute processing:
     - match namespace constraint first (independent of SymbolID)
     - processContents strict/lax/skip:
       - strict/lax attempt global attribute lookup by SymbolID
       - strict unresolved => error
       - lax unresolved => accept without type validation
       - skip => accept without validation
   - After scanning present attributes:
     - For each declared attribute use not present:
       - if `Use==AttrRequired`: error
       - if `Use!=AttrProhibited` and (`Default.Present` or `Fixed.Present`): apply default/fixed value to PSVI; `Fixed` behaves like a default if absent.
11. Push new element frame:
    - frame holds:
      - ElemID (declared element)
      - TypeID actualType
      - content model state (if complex with model)
      - nilled flag
      - text collector state (for simple/simpleContent)
      - identity constraint activation state

### 8.4 Character data and simple content

- For complex types with element-only content, non-whitespace character content is error; whitespace-only text is allowed but still counts as character content for default/fixed rules.
- For mixed content, character content is allowed but may still participate in identity constraints if selected via XPath (rare; field paths in subset do not select text nodes directly).
- For simple types and complex types with simpleContent:
  - collect character data into `textBuf` (reuse buffer)
  - on EndElement, validate the accumulated lexical value using the compiled validator and produce canonical bytes for:
    - fixed/default comparison
    - identity constraints (if field refers to that element)

### 8.5 EndElement semantics (default/fixed and nil)

Default/fixed application for elements follows XSD rules:

- Default/fixed are applied only when the element has **neither element nor character children** and is not nilled.
  - Note: whitespace-only text counts as character children; therefore defaults do not apply in that case.

For nilled elements:

- If nilled==true, element must have no element or character children; whitespace-only is not allowed.
- Skip text validation and default/fixed application.

Content model acceptance:

- On EndElement, if the element has a content model (DFA/NFA/All), the model MUST be in an accepting state; else error.

Identity constraints:

- On EndElement of scope element, finalize node tables and emit:
  - unique: duplicates are error; rows with any missing/nilled field are excluded
  - key: duplicates error; any missing/nilled field is an error
  - keyref: rows with any missing/nilled field are excluded; remaining rows must resolve
- Keyref resolution is limited to the subtree of the element where the constraints are applied.

---

## 9. Content model engine and Glushkov construction

### 9.1 Answer: “Why not use Glushkov construction anymore?”

We still use the **Glushkov / followpos** machinery as the core compilation step because:

- It produces an ε-free automaton directly from regular expressions.
- It aligns with XSD’s particle model (positions correspond to element occurrences).
- It supports efficient bitset representations for streaming validation.

What changes in this refactor:

- We stop treating Glushkov as an implicit, string/QName-based runtime. We compile to a **numeric runtime model**:
  - transitions keyed by SymbolID
  - transitions carry the validated ElemID (member element for substitution groups)
  - wildcard matching uses namespace constraints independent of SymbolID
- We optionally determinize into DFA tables for the hottest path; NFA/bitset simulation is a fallback when state explosion would be too large.

This is the throughput-optimal hybrid: deterministic table stepping for common models, bitset fallback for large models.

### 9.2 Determinization policy

- Build Glushkov NFA (positions + followpos bitsets).
- Determinize to DFA via subset construction with cap:

```go
type CompileLimits struct {
    MaxDFAStates uint32 // default: 4096
    MaxOccursLimit uint32 // default: 1_000_000
}
```

If determinization exceeds `MaxDFAStates`, store an NFA model and validate via bitset simulation.

### 9.3 UPA and wildcard overlap

Before compiling transitions:

- Run UPA checks (Unique Particle Attribution) to guarantee:
  - for a given state and input element name/namespace, at most one particle matches
  - wildcard overlaps that would cause ambiguity are rejected at schema-compile time

This allows DFA transitions to map each SymbolID to at most one ElemID and allows wildcard edges not to overlap.

---

## 10. Local and anonymous components (mandatory support)

### 10.1 ID assignment

We assign IDs to:

- all global types/elements/attributes
- all local element declarations
- all anonymous types (simple and complex)
- all attribute-group-contained local attributes that can be reused via group refs

Rule: IDs are assigned deterministically by document discovery order, then by stable AST traversal order.

Traversal order definition:

- For each schema document instance in loader order:
  - visit global declarations in document order
  - within each global declaration, visit nested local declarations in document order (preorder)
  - assign IDs as encountered

Anonymous types:

- `Type.Name = 0`
- still get TypeID and participate in derivation chain and IsDerivedFrom checks

Local elements:

- get ElemID and are referenced directly by content model particles
- do not appear in GlobalElements index

### 10.2 Late resolution and cycles

Resolution strategy:

1. Parse all docs into AST; assign IDs to all components (including local/anonymous) while building component graphs.
2. Resolve QName references using global registries:
   - forward references are supported because registries are built from all docs first
3. Detect illegal cycles:
   - type derivation cycles (base chain) => error
   - group reference cycles => error
   - attributeGroup reference cycles => error
   - substitution group cycles => error or handled by SCC; choose deterministic error:
     - This refactor chooses **error on cycles** to preserve clear semantics and simplify expansion.

---

## 11. Substitution groups (critical correctness wiring)

### 11.1 Compile-time expansion

Content model compilation expands substitution groups into explicit transitions:

- For each particle referencing head element H:
  - compute substitution closure members M = {H plus all members not blocked}
  - for each member element m in M:
    - add a transition keyed by SymbolID(m.Name) that yields ElemID(m)

Result: runtime stepping returns the actual matched member element declaration. This resolves the critical failure where members were validated against the head type.

### 11.2 Membership checks and blocking

Membership closure respects:

- element `block` constraints (substitution is blocked by `ElemBlockSubstitution`, and by extension/restriction when member type derivation uses those methods)
- type derivation constraints of member type vs head type
- abstract head elements are still matchable via members, but head itself cannot appear if abstract

---

## 12. Wildcard processing (critical correctness wiring)

### 12.1 Matching must work for unknown names

Even if `SymbolID == 0` (unknown name), wildcard matching must still succeed when namespace constraints allow it.

Therefore:

- wildcard matching operates on `(nsBytes, nsID)` and not solely on SymbolID.
- the content model stepper signature includes both.

### 12.2 Declared type resolution under wildcard processContents

Element wildcard strict/lax/skip semantics are fully specified in §8.3 step 3.

Attribute wildcard strict/lax/skip semantics mirror element behavior:

- strict: unresolved global attribute => error
- lax: unresolved => accept without type validation
- skip: accept without validation

Namespace constraint applies first.

---

## 13. Value compilation ordering (critical ordering fix)

The compile pipeline must not require validators that are compiled later.

Correct order (normative):

1. Build intern tables (Namespaces, Symbols).
2. Assign IDs and build component graphs.
3. Compile type derivation order (topological).
4. **Compile validators for simple types in derivation order**, including:
   - whitespace handling
   - regex compilation
   - numeric/date parsing setup
   - facet programs
   - enumeration canonicalization (requires validator partial compile; see below)
5. Compile defaults/fixed values for elements and attributes using already-compiled validators.
6. Compile content models.
7. Compile identity constraints (XPath programs and runtime linkages).
8. Freeze schema: no more mutation.

### 13.1 Enumeration canonicalization without circular dependency

For a simple type T derived by restriction:

- Compile a “partial validator” with all facets except enumeration.
- Canonicalize each enumeration lexical by:
  - applying whitespace normalization (from partial)
  - parsing to value space (from base primitive parser)
  - validating against partial facets
  - producing canonical bytes
- Build the enum set and finalize validator.

This resolves the “canonicalization depends on validators compiled later” inconsistency.

---

## 14. Error model (explicit, including value errors)

Validation must emit deterministic error codes.

### 14.1 Required additions

Add codes (examples; names must be stable constants):

- `VALIDATE_VALUE_INVALID` (type parse failed)
- `VALIDATE_VALUE_FACET` (facet violation: min/max/length/pattern/enum/digits)
- `VALIDATE_ELEMENT_ABSTRACT`
- `VALIDATE_SIMPLETYPE_ATTR_NOT_ALLOWED`
- `VALIDATE_XSI_TYPE_UNRESOLVED`
- `VALIDATE_XSI_TYPE_DERIVATION_BLOCKED`
- `VALIDATE_XSI_NIL_NOT_NILLABLE`
- `VALIDATE_NILLED_HAS_FIXED`
- `VALIDATE_NILLED_NOT_EMPTY`
- `VALIDATE_WILDCARD_ELEM_STRICT_UNRESOLVED`
- `VALIDATE_WILDCARD_ATTR_STRICT_UNRESOLVED`
- `VALIDATE_ROOT_NOT_DECLARED`

Where a W3C `cvc-*` code exists and the current library uses it, keep it. The new local codes are for gaps not covered.

---

## 15. Concrete implementation tasks (ordered for throughput first)

This task list is intentionally explicit. Each task is a concrete change-set that should compile and run tests.

### Phase A: Guardrails and scaffolding

1. Add benchmark suite reflecting workload:
   - `Benchmark_Validate_ManyDocs_OneSchema`
   - `Benchmark_Validate_SimpleTypes_FacetsHeavy`
   - `Benchmark_Validate_IdentityConstraints`
   - `Benchmark_Validate_DeepContentModel`
2. Add allocation guardrails:
   - success-path allocation benchmarks must fail CI if regression > threshold
   - include `pkg/xmlstream` and `pkg/xmltext` in guardrails (hot path)
3. Add conformance harness:
   - W3C suite runner wired into `go test` via build tags
   - differential tests vs libxml2/xerces for supported subset (optional but recommended)

### Phase B: Introduce Engine + Session (new API)

4. Create `xsd.Engine`, `xsd.Session`, `Compile*`, `Validate` APIs (new package surface).
5. Implement `Engine.pool sync.Pool` and session acquire/release.
6. Move current validator entrypoint to `Session.Validate` and make it goroutine-confined.

### Phase C: New runtime model package

7. Create `internal/runtime` package containing all structs defined in §7 (Schema, tables, bundles).
8. Define ID types and enforce ID=0 invalid across runtime.
9. Implement SymbolsTable + NamespaceTable interners with deterministic hashing:
   - pick one deterministic 64-bit hash with no new external deps (stdlib or in-tree)
   - guarantee hash!=0 by post-processing `if h==0 {h=1}`

### Phase D: Loader rewrite (deterministic + chameleon + import-without-location)

10. Replace loader with the normative behavior in §4:
    - directive scan preserves order
    - `(systemID, ETN)` cache key
    - strict path validation (reject `.`/`..` segments before cleaning)
    - import-without-location behavior exactly as specified
11. Define and implement `Resolver` interface and FS resolver.
12. Implement loader cycle handling with Unseen/Loading/Loaded states.

### Phase E: Parser rewrite (locals/anonymous + form defaults + QName context)

13. Redesign parsed AST:
    - store elementFormDefault and attributeFormDefault in schema AST
    - represent local elements, local attributes, anonymous types explicitly
    - represent identity constraints (`key`, `unique`, `keyref`) and their selector/field strings
14. Parser must maintain namespace contexts while reading schema docs and parse QName-valued attributes per §5.1.
15. Add explicit representation for simpleContent/complexContent and derivation method.

### Phase F: Resolver + schemacheck rewrite (IDs for all components + cycle detection)

16. Build registries:
    - global element/type/attribute registries by SymbolID
    - local component graphs by owning component
17. Assign IDs deterministically (globals + locals + anonymous) per §10.
18. Resolve all references:
    - `ref=...`, `type=...`, `base=...`, `substitutionGroup=...`, groups/attributeGroups
19. Detect cycles:
    - type base cycle
    - group ref cycle
    - attributeGroup cycle
    - substitution group cycle (treat as error)
20. Compute type ancestor chains + cumulative derivation masks (populate AncOff/AncLen/AncMaskOff).
21. Implement UPA check for content models including wildcard overlaps.

### Phase G: Validator compilation (ordering fix + full validator bundle)

22. Implement validator compiler per §13:
    - topological order by type derivation
    - partial validator for enum canonicalization
    - store facet programs and compiled regex
23. Implement `internal/value` parsing layer:
    - zero-allocation whitespace normalization
    - numeric/date/time/duration parsers (start with strconv/time.Parse, then optimize)
    - QName parser that accepts `NSResolver`
24. Implement ValueBlob builder and canonical representation for:
    - strings (maybe borrow input if WS_Preserve and no canonicalization needed)
    - numerics (canonical lexical)
    - QName (uri + 0 separator + local)
25. Compile default/fixed for elements and attributes after validators exist.

### Phase H: Content models (Glushkov + DFA + wildcard/subst correctness)

26. Implement Glushkov builder that emits:
    - positions
    - followpos bitsets
    - first/last/nullable
27. Determinize to DFA up to MaxDFAStates default=4096.
28. Expand substitution groups during compilation so DFA transitions store member ElemID.
29. Add wildcard edges as DFAWildcardEdge and wire processContents in validator stepper.
30. Implement explicit end-of-element acceptance checks.

### Phase I: Streaming validator runtime (throughput + correctness)

31. Rewrite validation as a Session state machine:
    - element stack frames contain current model state
    - avoid maps on hot path (use nameMap indexed by NameID)
32. Implement xsi:type retargeting before other attribute/content checks.
33. Implement xsi:nil emptiness rule (no character or element children).
34. Implement anyAttribute namespace constraints + strict/lax/skip behavior (namespace-based, independent of SymbolID).
35. Implement QName typed value validation using instance namespace context (inject NSResolver).
36. Ensure session resets clear nameMap per document.

### Phase J: Identity constraints (bytecode + streaming state)

37. Implement XPath compiler for the restricted grammar (§6) and produce PathPrograms.
38. Implement per-session identity evaluation:
    - start constraints by element scope
    - selector matches create rows
    - field evaluation collects canonical values
39. Implement key/unique duplicate detection with open-address tables keyed by canonical bytes hash + bytes.
40. Implement keyref resolution at scope end with deterministic error codes.

### Phase K: Cleanup and simplification (reduce duplication/long functions)

41. Delete DOM validation paths unless required for tests; keep only streaming where possible.
42. Collapse duplicated QName/fixed/simple-type helper code into `internal/value` + `internal/runtime`.
43. Replace interface-heavy code with switch-on-kind + slice bundles.
44. Split long functions in validator into:
    - token dispatch
    - start-element handling
    - attribute validation
    - content model feed
    - end-element finalize
45. Remove package-global mutable caches; all caches must live on Engine (immutable) or Session (reset).

---

## 16. Acceptance criteria

### 16.1 Throughput

- “Many docs, one schema” throughput improves materially (target: ≥2x vs current baseline).
- Allocations in success path approach zero.

### 16.2 Concurrency

- Validate N documents in N goroutines with linear-ish scaling to CPU saturation.
- No global locks contended in the hot path.

### 16.3 Correctness

- W3C tests pass for supported features.
- Key edge cases:
  - wildcard matching for unknown names works
  - substitution group member validated against member type
  - xsi:type retargeting happens before other validation
  - xsi:nil forbids any character children (including whitespace)
  - schema-time QName resolution follows in-scope namespaces and import constraints
  - restricted XPath subset is exactly the published grammar

---

## 17. Implementation checklist (quick)

If you want the “smallest viable fix set” to unblock correctness before full refactor, do these first:

1. Fix compile ordering: validators before value canonicalization.
2. Add locals/anonymous component support with deterministic IDs.
3. Add namespace-based wildcard matching independent of SymbolID.
4. Fully specify and implement schema QName resolution rules and instance QName validator context.
5. Clarify and implement include/import semantics including chameleon and import-without-location.



---

## Appendix A. Remaining runtime definitions (previously implicit)

This appendix makes the document fully self-contained: all referenced runtime types are defined here.

### A.1 Global indices (implementation detail)

Global indices are direct arrays:

- `Schema.GlobalElements[symbolID] -> ElemID`
- `Schema.GlobalTypes[symbolID] -> TypeID`
- `Schema.GlobalAttributes[symbolID] -> AttrID`

They are built at compile time by iterating the corresponding global components and writing into arrays sized `Symbols.Count()+1`.

### A.2 Patterns table

Patterns are compiled once at schema compile time.

```go
type PatternID uint32

type Pattern struct {
    Re *regexp.Regexp // Go's regexp is safe for concurrent use
}
```

Facet instructions reference patterns by `PatternID` stored in `FacetInstr.Arg0`.

### A.3 Value blob

All canonical value bytes live in one immutable blob.

```go
type ValueBlob struct {
    Blob []byte
}
```

`ValueRef.Off/Len` slice into `Blob`. `ValueRef.Hash` is a deterministic hash of those bytes.

### A.4 Enumeration table

Enumerations are represented as sets of canonical values.

```go
type EnumID uint32

type EnumTable struct {
    // For each EnumID: range into Values slice
    Off []uint32
    Len []uint32

    Values []ValueRef // canonical values (refs into Schema.Values.Blob)

    // Optional acceleration: for large enums, build per-enum open-address hash tables
    HashOff []uint32
    HashLen []uint32
    Hashes  []uint64
    Slots   []uint32 // index into Values (relative to enum Off)
}
```

Lookup algorithm is deterministic:

- If Len <= 16: linear scan comparing hashes then bytes
- Else: open-address hash table using `ValueRef.Hash` with collision check by bytes

### A.5 Union/list validators (runtime representation)

```go
type ListValidator struct {
    Item ValidatorID
}

type UnionValidator struct {
    MemberOff uint32 // into Schema.Validators.UnionMembers
    MemberLen uint32
}
```

Storage:

```go
type ValidatorsBundle struct {
    // ...
    UnionMembers []ValidatorID
    // ...
}
```

Union validation strategy:

- Validate lexically against each member in order.
- First member that validates wins.
- Canonical bytes are those produced by the winning member.

Determinism guarantee: member order is the schema’s `memberTypes` order (or restriction order), preserved exactly.

### A.6 NFA fallback model (Glushkov bitset simulation)

If DFA determinization exceeds MaxDFAStates, store an NFAModel.

```go
type BitsetRef struct {
    Off uint32 // into BitsetBlob.Words
    Len uint32 // number of uint64 words
}

type BitsetBlob struct {
    Words []uint64
}

type PosMatchKind uint8
const (
    PosExact PosMatchKind = iota
    PosWildcard
)

type PosMatcher struct {
    Kind PosMatchKind

    // For PosExact:
    Sym  SymbolID
    Elem ElemID

    // For PosWildcard:
    Rule WildcardID
}

type NFAModel struct {
    Bitsets BitsetBlob

    Start   BitsetRef // start positions set
    Accept  BitsetRef // accepting positions set
    FollowOff uint32  // into Follow slice
    FollowLen uint32

    Matchers []PosMatcher // per position index
    Follow   []BitsetRef  // per position: followpos bitset
}
```

Runtime stepping:

- Current NFA state is a bitset in session scratch (same word length as model bitsets).
- On input element:
  - compute `matchedPositions` as bitset of positions whose matcher accepts this element:
    - PosExact matches when Sym==symID
    - PosWildcard matches when wildcard rule accepts namespace (independent of symID)
  - intersect with current state, then OR followpos of each set bit to get next state
- Acceptance:
  - At end-of-element, current state must intersect Accept (or be nullable if model empty); otherwise error

This is slower than DFA but prevents state explosion.

### A.7 Occurrence parsing and overflow policy (unambiguous)

Occurrence values (`minOccurs`, `maxOccurs`) compile-time rules:

- Parse as base-10 uint32.
- `maxOccurs="unbounded"` maps to sentinel `MaxOccurs = math.MaxUint32`.
- Any numeric overflow while parsing => error `PARTICLES_OCCURS_OVERFLOW`.
- Any value > `CompileLimits.MaxOccursLimit` (compile option, default 1_000_000) => error `PARTICLES_OCCURS_OVERFLOW`.
- There is no “overflow becomes unbounded” behavior.

Error code differentiation:

- `SCHEMA_OCCURS_TOO_LARGE`: schema author explicitly used a value > CompileLimits.MaxOccursLimit (including huge literals).
- `PARTICLES_OCCURS_OVERFLOW`: parsing overflow or intermediate multiplication/addition overflow while normalizing ranges.

### A.8 xmlstream contract additions (hot path)

`xmlstream.Reader` MUST provide:

- StartElement event with:
  - a stable `NameID` for the element name (scope: current document)
  - namespace URI bytes for the element (already prefix-resolved)
  - local name bytes
  - attributes as a slice with per-attribute namespace URI bytes, local bytes, and value bytes
  - each attribute entry includes a stable `NameID` for its name (scope: current document)
  - namespace declarations must be represented in a way that Session can push them into nsStack before QName parsing

Duplicate attribute rule:

- Duplicates are defined by expanded name (namespace URI + local name), not by raw prefix.
- Either:
  - xmlstream detects duplicates and returns a well-formedness error, or
  - Session detects duplicates on the StartElement event and errors.
- This refactor chooses: detect in xmlstream to keep validator simpler and avoid repeated checks.

### A.9 Minor cleanup items captured from review

These are low risk but must be done for polish and consistency:

- Rename `refAttrGroup` constant to exported-consistent `RefAttrGroup`.
- Fix any path formatting inconsistencies in package layout documentation (e.g., `/internal/facets`).
- Remove any remaining “same as v2” placeholders; every behavior must be specified in this document.



---

## Normative references

This specification intentionally aligns with the following stable standards (section numbers refer to the published documents):

- W3C XML Schema 1.0 Part 1: Structures (Second Edition, 2004)
  - §4.2.1 (Schema Document Location Strategy: include/import constraints; chameleon include; import schemaLocation optionality)
  - §3.15.3 (QName interpretation and resolution in schema documents)
  - §3.3.4 (Element Locally Valid / xsi:nil emptiness rule)
  - §3.11.6 (Identity-constraint XPath subset grammar for selector and fields)
- W3C Namespaces in XML (QName and namespace name concepts)
