# xsd

Pure Go XML Schema 1.0 validator.

The public API is intentionally small:

- compile schemas once with `xsd.Compile`
- pass schema sources with `xsd.File` or `xsd.Reader`
- validate each XML document with `Engine.Validate`
- inspect failures with `errors.AsType[*xsd.Error]`

Validation is streaming. `Engine.Validate` consumes an `io.Reader`; it does not build a DOM or store the full instance document.

`File` resolves local `xs:include` and `xs:import` `schemaLocation` values relative to each schema file. `Reader` uses only sources passed to `Compile` unless paired with a `Resolver`. HTTP and network schema loading are not performed by default.

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
        MaxSchemaDepth:      256,
        MaxSchemaAttributes: 256,
        MaxSchemaTokenBytes: 4 << 20,
        MaxSchemaNames:      0,
        MaxFiniteOccurs:     1_000_000,
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
| `MaxSchemaNames` | `0` | Max interned schema names, including built-ins. `0` means no explicit limit. |
| `MaxFiniteOccurs` | `0` | Max accepted finite `maxOccurs`. `0` means no explicit limit. |

Negative integer limits are schema compile errors.

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
    wg.Add(1)
    go func() {
        defer wg.Done()
        errs <- engine.Validate(strings.NewReader(doc))
    }()
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

## Constraints

- XSD 1.0 only.
- Schema sources are explicit. No HTTP or network fetching.
- Instance documents must be UTF-8.
- DTDs and external entities are rejected.
- `xsi:schemaLocation` never triggers dynamic loading.
- Regex support is limited to patterns representable by Go `regexp`.
- Date/time values using BCE years or years outside `0001..9999` are unsupported for `xs:date` and `xs:dateTime`.
- `xs:redefine` is unsupported.

