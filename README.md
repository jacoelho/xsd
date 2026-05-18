# xsd

Pure Go XML Schema 1.0 validator.

The public API is intentionally small:

- compile schemas once with `xsd.Compile`
- pass schema sources with `xsd.File`, `xsd.Reader`, or `xsd.LimitedReader`
- validate each XML document with `Engine.Validate`
- reuse document-local state with `Engine.NewSession` when useful
- inspect failures with `errors.AsType[*xsd.Error]`

Validation is streaming. `Engine.Validate` consumes an `io.Reader`; it does not build a DOM or store the full instance document.

`File` resolves local `xs:include` and `xs:import` `schemaLocation` values relative to each schema file. `Reader` uses only sources passed to `Compile` unless paired with a `Resolver`. `Reader` eagerly reads the whole input so the source can be reused; use `LimitedReader` for untrusted reader inputs. HTTP and network schema loading are not performed by default.

## Install

```sh
go get github.com/jacoelho/xsd
```

## Compile From Reader

```go
schema := strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)

engine, err := xsd.Compile(xsd.Reader("schema.xsd", schema))
if err != nil {
    return err
}

err = engine.Validate(strings.NewReader(`<root>7</root>`))
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
schema := strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)

engine, err := xsd.CompileWithOptions(
    xsd.CompileOptions{
        MaxSchemaDepth:        256,
        MaxSchemaAttributes:   256,
        MaxSchemaTokenBytes:   4 << 20,
        MaxSchemaSourceBytes:  64 << 20,
        MaxSchemaNames:        0,
        MaxFiniteOccurs:       1_000_000,
        MaxContentModelStates: 16_384,
    },
    xsd.Reader("schema.xsd", schema),
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
| `MaxSchemaNames` | `0` | Max interned schema names, including built-ins. `0` means no explicit limit. |
| `MaxFiniteOccurs` | `0` | Max accepted finite `maxOccurs`. `0` uses the runtime `uint32` cap. |
| `MaxContentModelStates` | `16_384` | Max DFA states per compiled content model. |

Negative integer limits are schema compile errors.

`MaxSchemaSourceBytes` applies during compilation to every source, including files, resolver-loaded includes/imports, and data captured by `Reader`. Because `Reader` reads eagerly before `CompileWithOptions` runs, callers that need to cap untrusted `io.Reader` input should use `LimitedReader`:

```go
engine, err := xsd.Compile(xsd.LimitedReader("schema.xsd", r, 64<<20))
if err != nil {
    return err
}
```

Finite `minOccurs` and `maxOccurs` values above `4294967295` are schema compile errors. `MaxFiniteOccurs` can lower the finite `maxOccurs` limit, but it cannot raise it above the runtime `uint32` representation. `maxOccurs="unbounded"` is not affected by this cap.

## Validation Options

Use `ValidateWithOptions` for one validation call, or `NewSession` to reuse document-local buffers across calls:

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
| `MaxIdentityEntries` | `0` | Max stored ID, IDREF, key, unique, and keyref entries. `0` means unlimited. |
| `MaxIdentityTupleBytes` | `0` | Max byte length of one stored identity key. `0` means unlimited. |
| `MaxInstanceDepth` | `0` | Max nested XML elements. `0` means unlimited. |
| `MaxInstanceAttributes` | `0` | Max attributes on one XML element. `0` means unlimited. |
| `MaxInstanceTextBytes` | `0` | Max retained character data bytes. `0` means unlimited. |
| `MaxInstanceTokenBytes` | `0` | Max retained XML token payload bytes. `0` means unlimited. |

Negative integer limits are validation errors.

`Engine` is goroutine-safe. `Session` is not goroutine-safe; use one session per goroutine. `Session.Validate` resets document-local state before every validation.

## Resolve Includes From Reader

```go
type mapResolver map[string]string

func (r mapResolver) ResolveSchema(base, location string) (xsd.SchemaSource, error) {
    data, ok := r[location]
    if !ok {
        return xsd.SchemaSource{}, xsd.ErrSchemaNotFound
    }
    return xsd.Reader(location, strings.NewReader(data)), nil
}

schema := strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="Root"/>
</xs:schema>`)

engine, err := xsd.Compile(xsd.Reader("schema.xsd", schema).WithResolver(mapResolver{
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

if xerr, ok := errors.AsType[*xsd.Error](err); ok {
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

Use `xsd.IsUnsupported(err)` when only unsupported-feature detection matters.

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

## xmllint-Compatible CLI

The repository includes a small CLI for xmllint-style validation:

```sh
go run ./cmd/xmllint --noout --huge \
  --schema schema.xsd \
  document.xml
```

Available flags:

| Flag | Required | Meaning |
| --- | --- | --- |
| `--schema path` | yes | Schema file path. |
| `--noout` | no | Accepted for compatibility. Document output is always suppressed. |
| `--huge` | no | Accepted for compatibility. |
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
XSD_LARGE_COMPARE=1 go test -run TestLargeXMLLintComparison -timeout=0 -v
```

By default this generates streaming XML documents at `100MB`, `500MB`, `1GB`, and `2GB`, plus an identity-constraint document. Generated files use `t.TempDir()` and are removed after each subtest. Set `XSD_LARGE_DIR=/path/to/dir` to keep generated files. Set `XSD_LARGE_SIZE_BYTES=1048576` for a quick single-size smoke run.

The command comparison reports elapsed time and max RSS from `/usr/bin/time` (`-l` on Darwin, `-v` on Linux). Max RSS is process memory, not Go `allocs/op`.

Latest local run (2026-05-18, Go 1.26.2, libxml2 2.9.13):

```text
goos: darwin
goarch: arm64
pkg: github.com/jacoelho/xsd

                         | libxml2 xmllint |             go xmllint             |
                         | sec/op          | sec/op          vs base           |
streaming/100MB                   1.806s          3.321s      +83.92%
streaming/500MB                  11.204s         15.059s      +34.41%
streaming/1GB                    25.433s         30.583s      +20.25%
streaming/2GB                    51.891s         62.161s      +19.79%
identity                       615.367ms       897.544ms      +45.86%
geomean                           6.968s          9.688s      +39.03%

                         | libxml2 xmllint |             go xmllint             |
                         | rss/op          | rss/op          vs base           |
streaming/100MB                  1.17GiB        12.25MiB      -98.98%
streaming/500MB                  5.81GiB        12.88MiB      -99.78%
streaming/1GB                    9.50GiB        12.95MiB      -99.87%
streaming/2GB                   12.44GiB        13.56MiB      -99.89%
identity                       185.62MiB        72.98MiB      -60.68%
geomean                          2.71GiB        18.25MiB      -99.34%
```

## Constraints

- XSD 1.0 only.
- Schema sources are explicit. No HTTP or network fetching.
- `File` resolves local relative refs and absolute local `file:` URIs. For untrusted schemas, use `Reader` or `LimitedReader` with an explicit `WithResolver`.
- Instance documents must be UTF-8.
- DTDs and external entities are rejected.
- `xsi:schemaLocation` never triggers dynamic loading.
- `FormatXML` builds an in-memory formatting tree; validation is the streaming path.
- Regex support is limited to patterns representable by Go `regexp`; unsupported XSD constructs such as class subtraction, `\i`/`\c`, and Unicode block escapes fail closed with `unsupported.regex`.
- `xs:redefine` is unsupported.
