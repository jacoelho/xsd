# XSD 1.0 Validator for Go

XSD 1.0 validation for Go with `io/fs` schema loading and streaming XML validation.

## Install

```bash
go get github.com/jacoelho/xsd
```

## Quickstart

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

	schema, err := xsd.Compile(fsys, "simple.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions())
	if err != nil {
		fmt.Printf("Compile schema: %v\n", err)
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

## Phased API

Single-root compile:

```go
schema, err := xsd.Compile(fsys, "schema.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions())
```

File-based compile:

```go
schema, err := xsd.CompileFile("schema.xsd", xsd.NewSourceOptions(), xsd.NewBuildOptions())
```

Multi-root or reusable prepared build:

```go
set := xsd.NewSourceSet().
	WithSourceOptions(xsd.NewSourceOptions())

if err := set.AddFS(fsysA, "schema-a.xsd"); err != nil {
	// handle
}
if err := set.AddFS(fsysB, "schema-b.xsd"); err != nil {
	// handle
}

prepared, err := set.Prepare()
if err != nil {
	// handle
}

schema, err := prepared.Build(xsd.NewBuildOptions())
if err != nil {
	// handle
}
```

## Validation

Default validation:

```go
if err := schema.Validate(strings.NewReader(xmlDoc)); err != nil {
	// handle
}
```

Explicit validator configuration:

```go
validator, err := schema.NewValidator(
	xsd.NewValidateOptions().
		WithInstanceMaxDepth(512).
		WithInstanceMaxTokenSize(1 << 20),
)
if err != nil {
	// handle
}

if err := validator.Validate(strings.NewReader(xmlDoc)); err != nil {
	// handle
}
```

Validate files:

```go
if err := schema.ValidateFile("document.xml"); err != nil {
	// handle
}
if err := validator.ValidateFSFile(fsys, "document.xml"); err != nil {
	// handle
}
```

## Options

Source options control schema loading and schema XML parsing:

```go
sourceOpts := xsd.NewSourceOptions().
	WithAllowMissingImportLocations(true).
	WithSchemaMaxDepth(512)
```

Build options control immutable runtime compilation:

```go
buildOpts := xsd.NewBuildOptions().
	WithMaxDFAStates(4096).
	WithMaxOccursLimit(1_000_000)
```

Validate options control instance XML parsing and validator sessions:

```go
validateOpts := xsd.NewValidateOptions().
	WithInstanceMaxDepth(512).
	WithInstanceMaxAttrs(256).
	WithInstanceMaxTokenSize(1 << 20)
```

## Loading behavior

- `Compile` and `SourceSet` accept any `fs.FS`; include/import locations resolve relative to the including schema path.
- Includes must resolve successfully.
- Imports without `schemaLocation` are rejected unless `WithAllowMissingImportLocations(true)` is set.

## Validation behavior

- `Schema.Validate` is safe for concurrent use.
- Validation is streaming; the document is not loaded into a DOM.
- Instance-document schema hints (`xsi:schemaLocation`, `xsi:noNamespaceSchemaLocation`) are ignored.

## Error handling

`Schema.Validate` returns `errors.ValidationList` for validation failures, XML parsing failures, and validation calls made without a loaded schema.
`Schema.ValidateFile` can return file I/O errors before validation starts.

Each `errors.Validation` includes:

- `Code`
- `Message`
- `Path`
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

## CLI (`xmllint`)

```bash
make xmllint
./bin/xmllint --schema schema.xsd document.xml
```

Options:

- `--schema` (required): path to the XSD schema file
- `--cpuprofile`: write a CPU profile to a file
- `--memprofile`: write a heap profile to a file

## Testing

```bash
go test -timeout 60s ./...
make w3c
```

## License

MIT
