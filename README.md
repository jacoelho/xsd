# XSD 1.0 Validator for Go

XSD 1.0 validation for Go with io/fs schema loading and streaming XML validation.

## Install

```bash
go get github.com/jacoelho/xsd
```

## Quickstart (in-memory schema)

```go
package main

import (
    "fmt"
    "strings"
    "testing/fstest"

    "github.com/jacoelho/xsd"
    "github.com/jacoelho/xsd/errors"
)

func main() {
    schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/simple"
           elementFormDefault="qualified">
  <xs:element name="person">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="name" type="xs:string"/>
        <xs:element name="age" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

    fsys := fstest.MapFS{
        "simple.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
    }

    schema, err := xsd.Load(fsys, "simple.xsd")
    if err != nil {
        fmt.Printf("Load schema: %v\n", err)
        return
    }

    xmlDoc := `<?xml version="1.0"?>
<person xmlns="http://example.com/simple">
  <name>John Doe</name>
  <age>30</age>
</person>`

    if err := schema.Validate(strings.NewReader(xmlDoc)); err != nil {
        if violations, ok := errors.AsValidations(err); ok {
            for _, v := range violations {
                fmt.Println(v.Error())
            }
            return
        }
        fmt.Printf("Validate: %v\n", err)
        return
    }

    fmt.Println("Document is valid")
}
```

## Validate from files

```go
package main

import (
    "fmt"

    "github.com/jacoelho/xsd"
    "github.com/jacoelho/xsd/errors"
)

func main() {
    schema, err := xsd.LoadFile("schema.xsd")
    if err != nil {
        fmt.Printf("Load schema: %v\n", err)
        return
    }

    if err := schema.ValidateFile("document.xml"); err != nil {
        if violations, ok := errors.AsValidations(err); ok {
            for _, v := range violations {
                fmt.Println(v.Error())
            }
            return
        }
        fmt.Printf("Validate: %v\n", err)
        return
    }

    fmt.Println("Document is valid")
}
```

## Load options

```go
opts := xsd.NewLoadOptions().
    WithAllowMissingImportLocations(true).
    WithMaxDFAStates(4096).
    WithMaxOccursLimit(1_000_000)

schema, err := xsd.LoadWithOptions(fsys, "schema.xsd", opts)
```

Options:
- `WithAllowMissingImportLocations`: when true, imports without `schemaLocation` are skipped.
  Missing import files are also skipped when the filesystem returns `fs.ErrNotExist`.
- `WithMaxDFAStates`: limit DFA determinization size for content models. `0` uses the default.
- `WithMaxOccursLimit`: limit particle `maxOccurs` during compilation. `0` uses the default.
- `WithSchemaMaxDepth` / `WithSchemaMaxAttrs` / `WithSchemaMaxTokenSize`: schema parser XML limits.
- `WithInstanceMaxDepth` / `WithInstanceMaxAttrs` / `WithInstanceMaxTokenSize`: instance XML limits.

## Prepare Once, Build Many

```go
prepared, err := xsd.Prepare(fsys, "schema.xsd")
if err != nil {
    // handle
}

schemaA, err := prepared.Build()
if err != nil {
    // handle
}

runtimeOpts := xsd.NewRuntimeOptions().
    WithMaxDFAStates(2048).
    WithInstanceMaxDepth(512)
schemaB, err := prepared.BuildWithOptions(runtimeOpts)
if err != nil {
    // handle
}
```

## Loading behavior

- `Load` accepts any `fs.FS`; include/import locations resolve relative to the including schema path.
- Includes MUST resolve successfully.
- Imports without `schemaLocation` are rejected unless `WithAllowMissingImportLocations(true)` is set.

## Validation behavior

- `Schema.Validate` is safe for concurrent use.
- Validation is streaming; the document is not loaded into a DOM.
- Instance-document schema hints (`xsi:schemaLocation`, `xsi:noNamespaceSchemaLocation`) are ignored.

## Error handling

`Schema.Validate` returns `errors.ValidationList` for validation and XML parsing failures.
`Schema.ValidateFile` can return file I/O errors before validation starts.

Each `errors.Validation` includes:
- `Code` (W3C codes like `cvc-elt.1`, or local codes like `xsd-schema-not-loaded`)
- `Message`
- `Path` (best-effort instance path)
- `Line` and `Column` when available
- `Expected` and `Actual` when available

## Constraints and limits

- XSD 1.0 only.
- No HTTP imports.
- Regex patterns must be compatible with Go's `regexp`.
- `xs:redefine` is not supported.
- DateTime parsing uses `time.Parse` (years 0001-9999; no year 0, BCE, or >9999).
- DTDs and external entity resolution are not supported.
- Instance documents must be UTF-8.

## CLI (xmllint)

```bash
make xmllint
./bin/xmllint --schema schema.xsd document.xml
```

Options:
- `--schema` (required): path to the XSD schema file

## Testing

```bash
go test -timeout 60s ./...
make w3c
```

## Architecture

See `docs/architecture.md`.

## License

MIT
