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

	schema, err := xsd.CompileFS(fsys, "simple.xsd", xsd.CompileConfig{})
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
		if violations, ok := xsd.AsValidations(err); ok {
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

## Compile API

Single-root compile:

```go
schema, err := xsd.CompileFS(fsys, "schema.xsd", xsd.CompileConfig{})
```

File-based compile:

```go
schema, err := xsd.CompileFile("schema.xsd", xsd.CompileConfig{})
```

Multi-root compile:

```go
compiler := xsd.NewCompiler(xsd.CompileConfig{
	Source: xsd.SourceConfig{AllowMissingImportLocations: true},
})
schema, err := compiler.CompileSources([]xsd.Source{
	{FS: fsysA, Path: "schema-a.xsd"},
	{FS: fsysB, Path: "schema-b.xsd"},
})
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
	xsd.ValidateConfig{XML: xsd.XMLConfig{
		MaxDepth:     512,
		MaxTokenSize: 1 << 20,
	}},
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

## Config

Zero-value config uses defaults. Set only limits or policies that need to differ.

```go
schema, err := xsd.CompileFS(fsys, "schema.xsd", xsd.CompileConfig{
	Source: xsd.SourceConfig{
		AllowMissingImportLocations: true,
		XML: xsd.XMLConfig{
			MaxDepth: 512,
		},
	},
	Build: xsd.BuildConfig{
		MaxDFAStates: 4096,
	},
	Validate: xsd.ValidateConfig{
		XML: xsd.XMLConfig{
			MaxAttrs:     256,
			MaxTokenSize: 1 << 20,
		},
	},
})
```

## Loading behavior

- `CompileFS` and `Compiler.CompileSources` accept any `fs.FS`; include/import locations resolve relative to the including schema path.
- `CompileFile` loads the explicit entry path as requested and confines nested include/import resolution to that path's containing directory tree.
- Includes must resolve successfully.
- Imports without `schemaLocation` are rejected unless `SourceConfig.AllowMissingImportLocations` is set.

## Validation behavior

- `Schema.Validate` and `Validator.Validate` are safe for concurrent use.
- Validation is streaming; the document is not loaded into a DOM.
- Instance-document schema hints (`xsi:schemaLocation`, `xsi:noNamespaceSchemaLocation`) are ignored.

## Error handling

`Schema.Validate` and `Validator.Validate` return `xsd.ValidationList` for validation failures and XML parsing failures.
Caller, compile, I/O, and internal failures return classified `xsd.Error` values.
`ValidateFile` and `ValidateFSFile` return `KindIO`/`ErrIO` for file errors before validation starts.

Each `xsd.Validation` includes:

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
