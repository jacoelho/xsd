# xsd

Pure Go XML Schema 1.0 validator.

The public API is intentionally small:

- compile schemas once with `xsd.Compile`
- pass reusable schema sources with `xsd.File`, `xsd.Bytes`, or `xsd.Open`
- validate each XML document with `Engine.Validate`
- reuse document-local state with `Engine.NewSession` when useful
- inspect failures with `errors.AsType[*xsderrors.Error]`

Validation is streaming. `Engine.Validate` consumes an `io.Reader`; it does not build a DOM or store the full instance document.

`File` resolves local `xs:include` and `xs:import` `schemaLocation` values relative to each schema file, including inherited `xml:base`. XSD 1.0 extended URI references are validated after XLink escaping: a custom resolver receives the whitespace-normalized, unescaped location and composed base, while built-in generic and file fallback uses the escaped URI projection. A resolver success is authoritative. Fragment-bearing locations are offered to a custom resolver; built-in file and generic identity resolution cannot interpret fragments and treat those optional hints as unresolved. Arbitrary source names remain identities rather than being reinterpreted as URI references, including Unix paths containing `#` or `?`. `Bytes` copies caller-owned schema bytes into a reusable source. `Open` calls a repeatable opener during compilation, so schema byte limits govern the first read. `Bytes` and `Open` use only sources passed to `Compile` unless paired with a `Resolver`; a resolver-returned source must have a non-empty name, which becomes that document's identity. HTTP and network schema loading are not performed by default.

## Install

```sh
go get github.com/jacoelho/xsd
```

Import the package with the `xsd` alias:

```go
import xsd "github.com/jacoelho/xsd"
```

Diagnostics live in the `xsderrors` package:

```go
import "github.com/jacoelho/xsd/xsderrors"
```

## Compile From Open

```go
schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int"/>
</xs:schema>`

engine, err := xsd.Compile(xsd.Open("schema.xsd", func() (io.ReadCloser, error) {
    return io.NopCloser(strings.NewReader(schema)), nil
}))
if err != nil {
    return err
}

err = engine.Validate(strings.NewReader(`<root>7</root>`))
if err != nil {
    return err
}
```

## Compile From Bytes

```go
schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)

engine, err := xsd.Compile(xsd.Bytes("schema.xsd", schema))
if err != nil {
    return err
}
```

## Compile From File

```go
engine, err := xsd.Compile(xsd.File("schema.xsd"))
if err != nil {
    return err
}

f, err := os.Open("document.xml")
if err != nil {
    return err
}
defer f.Close()

err = engine.Validate(f)
if err != nil {
    return err
}
```

## Compile Options

Use `CompileWithOptions` to override schema compile limits:

```go
schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)

engine, err := xsd.CompileWithOptions(
    xsd.CompileOptions{
        MaxSchemaDepth:             256,
        MaxSchemaAttributes:        256,
        MaxSchemaTokenBytes:        4 << 20,
        MaxSchemaSourceBytes:       64 << 20,
        MaxSchemaSources:           1024,
        MaxSchemaTotalBytes:        256 << 20,
        MaxSchemaReferences:        16_384,
        MaxSchemaTargetContexts:    4096,
        MaxSchemaInstantiatedNodes: 1_000_000,
        MaxSchemaNames:             0,
        MaxFiniteOccurs:            1_000_000,
        MaxContentModelStates:             16_384,
        MaxSubstitutionClosureEntries:     1_000_000,
        MaxSimpleUnionMemberEntries:       1_000_000,
    },
    xsd.Bytes("schema.xsd", schema),
)
if err != nil {
    return err
}
```

Available options:

| Option | Default | Meaning |
| --- | ---: | --- |
| `MaxSchemaDepth` | `256` | Max nested schema XML elements. |
| `MaxSchemaAttributes` | `256` | Max attributes on one schema XML element. |
| `MaxSchemaTokenBytes` | `4 << 20` | Max retained schema XML token payload. |
| `MaxSchemaSourceBytes` | `64 << 20` | Max bytes read from each schema source. |
| `MaxSchemaSources` | `1024` | Max explicit source descriptors and distinct resolver-loaded source identities admitted to one compilation. |
| `MaxSchemaTotalBytes` | `256 << 20` | Max aggregate bytes read across all schema sources. |
| `MaxSchemaReferences` | `16_384` | Max include/import references processed across the schema set. |
| `MaxSchemaTargetContexts` | `4096` | Max distinct source/effective-target-namespace contexts, including primary and chameleon-derived contexts. |
| `MaxSchemaInstantiatedNodes` | `1_000_000` | Max aggregate raw schema nodes across all target contexts. |
| `MaxSchemaNames` | `0` | Max interned schema names, including built-ins. `0` means no explicit limit. |
| `MaxFiniteOccurs` | `0` | Max accepted finite `maxOccurs`. `0` uses the runtime `uint32` cap. |
| `MaxContentModelStates` | `16_384` | Max DFA states per compiled content model. |
| `MaxSubstitutionClosureEntries` | `1_000_000` | Max aggregate transitive substitution-group relationships. |
| `MaxSimpleUnionMemberEntries` | `1_000_000` | Max aggregate flattened simple-union members. |

Negative integer limits are schema compile errors.

`MaxSchemaSourceBytes` applies to each source. `MaxSchemaSources` bounds both the explicit source-descriptor count before conversion and the distinct identities admitted from the resolver-expanded graph; repeated resolver references remain bounded by `MaxSchemaReferences` and `MaxSchemaTotalBytes`. `MaxSchemaTargetContexts` and `MaxSchemaInstantiatedNodes` bound derived target-namespace variants. `MaxSubstitutionClosureEntries` and `MaxSimpleUnionMemberEntries` bound derived compilation structures before immutable runtime lookups are published. These limits cover files, resolver-loaded includes/imports, `Bytes` data, and streams acquired by `Open`. `Open` must return a new independent reader on every call so the source remains retryable and safe for concurrent compilation:

```go
engine, err := xsd.Compile(xsd.Open("schema.xsd", func() (io.ReadCloser, error) {
    return openSchema()
}))
if err != nil {
    return err
}
```

Finite `minOccurs` and `maxOccurs` values above `4294967295` are schema compile errors. `MaxFiniteOccurs` can lower the finite `maxOccurs` limit, but it cannot raise it above the runtime `uint32` representation. `maxOccurs="unbounded"` is not affected by this cap.

## Validation Options

Use `ValidateWithOptions` for one validation call, or `NewSession` to reuse document-local buffers and bounded string caches across calls:

```go
session, err := engine.NewSession(xsd.ValidateOptions{
    MaxErrors:             1,
    MaxIdentityScopes:     10_000,
    MaxIdentityEntries:    100_000,
    MaxIdentityTupleBytes: 4 << 10,
})
if err != nil {
    return err
}

for _, doc := range docs {
    if err := session.Validate(strings.NewReader(doc)); err != nil {
        return err
    }
}
```

Available validation options:

| Option | Default | Meaning |
| --- | ---: | --- |
| `MaxErrors` | `0` | Max collected recoverable validation errors. `0` means unlimited. |
| `MaxIdentityScopes` | `0` | Max active identity-constraint scopes. `0` means unlimited. |
| `MaxIdentityEntries` | `0` | Max stored ID, IDREF, key, unique, and keyref entries and simultaneously pending identity-selector matches. `0` means unlimited. |
| `MaxIdentityTupleBytes` | `0` | Max byte length of one stored identity key. `0` means unlimited. |
| `MaxSchemaLocationNamespaces` | `256` | Max distinct schema-location namespace names retained per document. `0` selects this finite default. |
| `MaxSchemaLocationNamespaceBytes` | `64 KiB` | Max aggregate bytes in distinct retained schema-location namespace names. `0` selects this finite default; `MaxInstanceTokenBytes` separately bounds each complete hint attribute. |
| `MaxInstanceDepth` | `0` | Max nested XML elements. `0` means unlimited. |
| `MaxInstanceAttributes` | `0` | Max attributes on one XML element. `0` means unlimited. |
| `MaxInstanceTextBytes` | `0` | Max retained character data bytes. `0` means unlimited. |
| `MaxInstanceTokenBytes` | `0` | Max parser-owned bytes for one XML token, including retained payload and active construction scratch. `0` means unlimited. |

Negative integer limits are validation errors.

`Engine` is goroutine-safe. Copies of a `Session` refer to the same reusable state, and overlapping calls fail with `xsderrors.CodeValidationSession` before consuming the second input. Use separately constructed sessions for concurrent validation. `Session.Validate` clears document state before returning from each call but may retain bounded scratch buffers and small string caches; discard the session to release retained cache contents.

## Resolve Includes From Bytes

```go
type mapResolver map[string]string

func (r mapResolver) ResolveSchema(base, location string) (xsd.SchemaSource, error) {
    data, ok := r[location]
    if !ok {
        return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
    }
    return xsd.Bytes(location, []byte(data)), nil
}

schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="Root"/>
</xs:schema>`)

engine, err := xsd.Compile(xsd.Bytes("schema.xsd", schema).WithResolver(mapResolver{
    "types.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Root"><xs:sequence/></xs:complexType>
</xs:schema>`,
}))
if err != nil {
    return err
}

err = engine.Validate(strings.NewReader(`<root/>`))
if err != nil {
    return err
}
```

## Inspect Errors

```go
err := engine.Validate(strings.NewReader(`<root>x</root>`))

if xerr, ok := errors.AsType[*xsderrors.Error](err); ok {
    fmt.Println(xerr.Category)
    fmt.Println(xerr.Code)
    fmt.Println(xerr.Line, xerr.Column)
    fmt.Println(xerr.Path)
}
```

Error categories:

- `schema_parse`
- `schema_compile`
- `unsupported`
- `validation`
- `internal`

Use `xsderrors.IsUnsupported(err)` when only unsupported-feature detection matters.

## Reuse Engine Concurrently

`Engine` is immutable after compile. Share it across goroutines. `Validate` creates isolated per-document state for each call.

```go
docs := []string{`<root>1</root>`, `<root>2</root>`, `<root>3</root>`}

var wg sync.WaitGroup
errs := make(chan error, len(docs))
for _, doc := range docs {
    wg.Go(func() {
        errs <- engine.Validate(strings.NewReader(doc))
    })
}
wg.Wait()
close(errs)

for err := range errs {
    if err != nil {
        return err
    }
}
```

## xmllint-style CLI

The repository includes a small CLI for xmllint-style validation:

```sh
go run ./cmd/xmllint --schema schema.xsd \
  document.xml
```

Available flags:

| Flag | Required | Meaning |
| --- | --- | --- |
| `--schema path` | yes | Schema file path. |
| `--max-errors n` | no | Maximum validation errors to collect. `0` means unlimited. |

## Benchmark Against libxml2

Build the Go `xmllint` binary into `bin`, and make sure libxml2 `xmllint` resolves from `PATH`:

```sh
make xmllint
command -v xmllint
```

`command -v xmllint` must not point at `./bin/xmllint`; the benchmark compares `bin/xmllint` with the libxml2 binary from `PATH`.

Run full comparison:

```sh
XSD_LARGE_COMPARE=1 XSD_LARGE_RUNS=20 go test ./tests -run TestLargeXMLLintComparison -timeout=0 -v
```

By default this generates streaming XML documents at `20MB`, `100MB`, `500MB`, `1GB`, and `2GB`, plus an identity-constraint document. Each command runs 20 times per profile and the tables report nearest-rank p95. Generated files use `t.TempDir()` and are removed after each subtest. Set `XSD_LARGE_DIR=/path/to/dir` to keep generated files. Set `XSD_LARGE_SIZE_BYTES=1048576 XSD_LARGE_RUNS=1` for a quick single-size smoke run.

The command comparison reports p95 elapsed time and p95 max RSS from `/usr/bin/time` (`-l` on Darwin, `-v` on Linux). Max RSS is process memory, not Go `allocs/op`.

Historical local run (2026-06-17, macOS 26.5, Go 1.26.4, libxml2 2.9.13, `main`, p95 over 20 runs):

```text
goos: darwin
goarch: arm64
pkg: github.com/jacoelho/xsd

                         | libxml2 xmllint |             go xmllint             |
                         | p95 sec/op      | p95 sec/op      vs base           |
streaming/20MB                 400.405ms       348.812ms      -12.89%
streaming/100MB                   1.792s          1.685s       -5.98%
streaming/500MB                  13.447s          8.336s      -38.01%
streaming/1GB                    26.104s         16.685s      -36.08%
streaming/2GB                    52.113s         33.715s      -35.30%
identity                       619.045ms       212.698ms      -65.64%
geomean                           4.484s          2.893s      -35.48%

                         | libxml2 xmllint |             go xmllint             |
                         | p95 rss/op      | p95 rss/op      vs base           |
streaming/20MB                 243.19MiB         6.97MiB      -97.13%
streaming/100MB                  1.17GiB         7.03MiB      -99.41%
streaming/500MB                  5.63GiB         7.34MiB      -99.87%
streaming/1GB                    8.45GiB         7.33MiB      -99.92%
streaming/2GB                   12.32GiB         7.53MiB      -99.94%
identity                       188.55MiB        69.73MiB      -63.01%
geomean                          1.76GiB        10.56MiB      -99.42%
```

## Constraints

- XSD 1.0 only.
- Schema sources are explicit. No HTTP or network fetching.
- `File` resolves local relative refs, inherited `xml:base`, and absolute local `file:` URIs. Use `Open` for repeatable reader-backed schemas whose first read must be compiler-bounded.
- Instance documents must be UTF-8.
- DTDs and external entities are rejected.
- `xsi:schemaLocation` never triggers dynamic loading.
- The repository XML formatter builds an in-memory formatting tree; validation is the streaming path.
- Regex support uses Go `regexp` plus a simple literal/class fast path for exact, bounded, and open repeats. Unsupported XSD constructs such as class subtraction, `\i`/`\c`, and Unicode block escapes fail closed with `unsupported.regex`.
- `xs:redefine` is unsupported.
