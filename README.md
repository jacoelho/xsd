# XSD 1.0 Validator for Go

Pure Go implementation of XSD 1.0 (XML Schema) validation with streaming and io/fs.

## Features
- Pure Go implementation
- io/fs integration for flexible schema loading
- Streaming validation with constant memory for large documents
- Built-in type validators for all XSD primitive and derived types
- Facet validation (pattern, enumeration, length, min/max, digits)
- Complex type validation with content models
- Sequence, choice, and all group validation
- Attribute validation with use and default/fixed value handling
- W3C error codes for validation failures

## Design decisions and limits
- XSD 1.0 only
- No support for HTTP imports
- Only regex patterns compatible with Go's regexp package
- xs:redefine is not supported
- DateTime types use time.Parse (years 0001-9999; no year 0, BCE dates, or years >9999)
- Instance-document schema hints (xsi:schemaLocation, xsi:noNamespaceSchemaLocation) are ignored

W3C test suite results are tracked in `w3c/w3c_test.go`. Run `make w3c` to measure conformance.

## Install

```bash
go get github.com/jacoelho/xsd
```

## Schema loading behavior
- `Load` accepts any `fs.FS`; include/import locations resolve relative to the including schema path.
- Missing include/import files are ignored when the filesystem returns `fs.ErrNotExist`.

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

## CLI (xmllint)

```bash
make xmllint
./bin/xmllint --schema schema.xsd document.xml
```

Options:
- `--schema` (required): path to the XSD schema file

## Error handling
- `Schema.Validate` returns `errors.ValidationList` for validation and XML parsing failures.
- `Schema.ValidateFile` can return file I/O errors before validation starts.

Each `errors.Validation` includes:
- `Code` (W3C codes like `cvc-elt.1`, or local codes like `xsd-schema-not-loaded`)
- `Message`
- `Path` (best-effort instance path)
- `Line` and `Column` when available
- `Expected` and `Actual` when available

## Security considerations

- Instance documents must be UTF-8. Non-UTF-8 encodings are rejected because xsd does not configure a charset reader.
- DTDs and external entity resolution are not supported.
- Limit input size for untrusted XML, for example with `io.LimitReader`.
- Schema selection is explicit; instance hints are ignored.

## Testing

```bash
go test -timeout 60s ./...
make w3c
```

## Architecture

See [README](./docs/README.md)

## License

MIT
