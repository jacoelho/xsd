# XSD 1.0 Validator for Go

Pure Go implementation of XSD 1.0 (XML Schema Definition) validation.


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

W3C test suite status: 51,119 pass, 868 skipped

## Design Decisions

- No support for HTTP imports
- Only regex patterns compatible with Go's `regexp` package
- `xs:redefine` is not supported
- DateTime types use Go's `time.Parse` (years 0001-9999; no year 0, BCE dates, or years >9999)


## Architecture

See docs/architecture.md for detailed documentation.

## Installation

```bash
go get github.com/jacoelho/xsd
```

## Usage

Load schema from file and validate:

```go
package main

import (
    "fmt"
    "os"

    "github.com/jacoelho/xsd"
    "github.com/jacoelho/xsd/errors"
)

func main() {
    schema, err := xsd.LoadFile("schemas/order.xsd")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to load schema: %v\n", err)
        os.Exit(1)
    }

    if err := schema.ValidateFile("order.xml"); err != nil {
        violations, ok := errors.AsValidations(err)
        if !ok {
            fmt.Fprintf(os.Stderr, "Failed to validate: %v\n", err)
            os.Exit(1)
        }

        fmt.Println("Validation errors:")
        for _, v := range violations {
            fmt.Printf("  [%s] %s at %s\n", v.Code, v.Message, v.Path)
        }
        os.Exit(1)
    }

    fmt.Println("Document is valid")
}
```

Load from io/fs:

```go
import (
    "fmt"
    "os"

    "github.com/jacoelho/xsd"
    "github.com/jacoelho/xsd/errors"
)

schema, err := xsd.Load(os.DirFS("./schemas"), "order.xsd")
if err != nil {
    fmt.Printf("Failed to load schema: %v\n", err)
}
if err := schema.Validate(xmlReader); err != nil {
    violations, ok := errors.AsValidations(err)
    if ok {
        for _, v := range violations {
            fmt.Printf("  [%s] %s\n", v.Code, v.Message)
        }
    }
}
```

Load from embedded filesystem:

```go
import (
    "embed"
    "github.com/jacoelho/xsd"
)

//go:embed schemas/*.xsd
var schemas embed.FS

schema, err := xsd.Load(schemas, "schemas/order.xsd")
```

Validation options (schemaLocation hints):

```go
opts := xsd.ValidateOptions{
    SchemaLocationPolicy: xsd.SchemaLocationDocument,
}
if err := schema.ValidateWithOptions(xmlReader, opts); err != nil {
    // handle validation errors
}
```

Policies:
- `SchemaLocationRootOnly` (default): only root element hints are applied.
- `SchemaLocationDocument`: pre-scans the document for hints; requires a
  seekable reader when hints are present.
- `SchemaLocationIgnore`: ignore all schemaLocation hints.

## Testing

Run all tests:

```bash
go test -timeout 60s ./...
```

Run with race detector:

```bash
go test -timeout 60s -race ./...
```

Run W3C conformance tests:

```bash
make w3c
```

## License

MIT
