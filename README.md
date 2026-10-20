# xsd

A pure Go XML Schema 1.0 validator. Compile a schema once, then validate XML
from an `io.Reader` without building a document tree or storing the full XML.

Requires Go 1.27 or newer. Accepts UTF-8 XSD 1.0 and XML 1.0. DTDs, external
entities, XML 1.1, and `xs:redefine` are unsupported. Schema sources are explicit:
the library does not fetch schemas over the network, and instance
`xsi:schemaLocation` hints never supply components. Validation uses the compiled
component set and reports missing required declarations or types as validation
errors. Hint syntax and resource limits still apply.

## Install and validate

For a new program, create a directory and run `go mod init example.com/validate`
there first. In your Go module, run:

```sh
go get github.com/jacoelho/xsd
```

Save this as `main.go`. The schema declares a `root` element containing an
integer; the XML supplies the value `7`.

```go
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/jacoelho/xsd"
)

func main() {
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int"/>
</xs:schema>`)

	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", schema))
	if err != nil {
		log.Fatal(err)
	}
	if err := engine.Validate(strings.NewReader(`<root>7</root>`)); err != nil {
		log.Fatal(err)
	}
	fmt.Println("valid")
}
```

Run `go run .`; it prints `valid`. Replace `7` with `hello` and validation
returns an error because the text is not an integer. Compilation checks the
schema; validation checks one XML document against it. A `nil` validation error
means the document is valid.

## Validate files

Use `xsd.File` to compile a local schema. It resolves local includes and imports
relative to the schema file, respecting `xml:base` and local `file:` URIs.
The caller opens and closes the XML document:

```go
import (
	"os"

	"github.com/jacoelho/xsd"
)

func validateFile(schemaPath, documentPath string) error {
	engine, err := xsd.Compile(xsd.File(schemaPath))
	if err != nil {
		return err
	}
	f, err := os.Open(documentPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return engine.Validate(f)
}
```

This and the following Go examples are helper functions. Add them to your
package with the imports shown, merging any existing imports.

## Handle errors

Compilation and validation return errors for unsupported features, malformed
input, and resource limits as well as schema violations. To print every returned
diagnostic:

```go
import (
	"fmt"

	"github.com/jacoelho/xsd/xsderrors"
)

func printDiagnostics(err error) {
	for _, diagnostic := range xsderrors.Flatten(err) {
		fmt.Println(diagnostic)
	}
}
```

`xsderrors.Error` has methods for category, code, message, path, line, and
column. `Message()` omits the cause; `Error()` includes it. Multiple recoverable
errors use `xsderrors.Errors`, with `Len()` and `At()` accessors.
Use `xsderrors.IsUnsupported(err)` to detect unsupported features.

For a non-nil error, `xsderrors.IsConclusiveValidation(err)` returns true only
when every independent diagnostic branch establishes a schema violation.
Malformed XML and resource failures return false. False can also mean there is
not enough evidence to classify the error; it does not mean the XML is valid.

## Load schemas from memory or streams

`xsd.Bytes(name, data)` copies the schema bytes. Pass multiple sources to
`Compile` when the schemas are already available:

```go
import "github.com/jacoelho/xsd"

func compileSchemas(root, types []byte) (*xsd.Engine, error) {
	return xsd.Compile(
		xsd.Bytes("schema.xsd", root),
		xsd.Bytes("types.xsd", types),
	)
}
```

For example, `root` can include `types.xsd` with
`<xs:include schemaLocation="types.xsd"/>`. Source names identify documents.
`Bytes` and `Open` resolve references from explicitly supplied sources unless
you attach a resolver.

Use `xsd.Open(name, opener)` for a stream, where `opener` has type
`func() (io.ReadCloser, error)`. It must return a new independent reader on
every call. Compilation applies byte limits from the first read and closes
every non-nil reader returned by the opener, including one returned with an
error. Close errors are reported; the caller must not reuse that reader.

To supply include/import dependencies on demand, attach a resolver. This example
loads `types.xsd` from a flat map:

```go
import (
	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func compileIncludes() (*xsd.Engine, error) {
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="types.xsd"/>
  <xs:element name="root" type="Root"/>
</xs:schema>`)
	sources := map[string]string{
		"types.xsd": `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Root"><xs:sequence/></xs:complexType>
</xs:schema>`,
	}
	resolver := xsd.ResolverFunc(func(base, location string) (xsd.SchemaSource, error) {
		data, ok := sources[location]
		if !ok {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return xsd.Bytes(location, []byte(data)), nil
	})
	return xsd.Compile(xsd.Bytes("schema.xsd", schema).WithResolver(resolver))
}
```

The resulting engine accepts `<root/>`. The map uses only `location`; a resolver
for nested paths should also use `base`.

Return `xsderrors.ErrSchemaNotFound` for a missing location to allow fallback
resolution. Wrapping a miss or joining only misses also allows fallback. Any
other error, including one joined with a miss, stops compilation. A successful
result is authoritative and must have a non-empty source name. The attached
resolver handles all descendants; a returned source's own resolver is ignored.

Supply XML and XLink attribute schemas through these same sources or resolver.
The library has no implicit `xml.xsd` or `xlink.xsd`; their namespace URIs never
trigger HTTP retrieval. For example, an import without `schemaLocation` can use
an explicitly supplied second root, as in `compileSchemas` above. The referring
schema must import the namespace. Supplied declarations control attribute types,
facets and fixed/default constraints. The same applies to XML attributes such as
`xml:lang` and `xml:space`; the predefined `xml` prefix does not replace a schema
import.

The four XSI attributes are intrinsic: `xsi:type` is QName-valued, `xsi:nil` is
boolean-valued, `xsi:noNamespaceSchemaLocation` is anyURI-valued, and
`xsi:schemaLocation` is a list of anyURI values. XSD and XSI references need no
import; supplied schemas cannot replace builtin declarations. Custom components
still require explicit schema sources. XSI defaults/fixed values do not supply
missing attributes; required uses still demand physical presence.

The supported language is XSD 1.0 with approved errata and explicitly
[selected corrections](docs/spec/SOURCES.md#selected-corrections). These include
typed fixed-attribute equality, preservation of restricted-union facets, and
qualified nilled identity-field handling. The
[correction audit](docs/spec/XSD11_CORRECTIONS.md) records the boundaries;
this does not enable XSD 1.1 syntax or schema HTTP loading.

## Reuse an engine or session

An engine is safe to share across goroutines. Each validation call has separate
document state. The engine retains at most one idle session for reuse;
concurrent calls acquire separate state without waiting. Engine copies share
that cache.

For sequential validation with fixed options and reusable buffers, create a
session:

```go
import (
	"strings"

	"github.com/jacoelho/xsd"
)

func validateDocuments(engine *xsd.Engine, docs []string) error {
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		return err
	}
	for _, doc := range docs {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			return err
		}
	}
	return nil
}
```

A session clears document state after each call but retains bounded buffers and
caches. Discard it to release them. Session copies share state: overlapping calls
fail with `xsderrors.CodeValidationSession` before reading the second input.
Use `Engine.Validate` or separate sessions for concurrent work.

## Resource limits

Defaults include 64 MiB per schema source, 256 MiB across schema sources,
64 MiB per XML document, and 100 recoverable validation errors. Set validation
limits for one call with `ValidateWithOptions`, or for a session with
`NewSession`:

```go
import (
	"io"

	"github.com/jacoelho/xsd"
)

func validateWithLimits(engine *xsd.Engine, doc io.Reader) error {
	return engine.ValidateWithOptions(doc, xsd.ValidateOptions{
		MaxErrors:        1,
		MaxInstanceBytes: 8 << 20,
	})
}
```

Reaching `MaxErrors` stops schema assessment, but XML is still read until the
end or a fatal error. A later fatal error takes precedence.

Compilation and validation are synchronous. Limits cannot interrupt a blocked
opener, resolver, file operation, or reader. Provide I/O you can close or
otherwise unblock if you need interruption.

See [resource limits](RESOURCE_LIMITS.md) for all defaults, zero-value rules,
and a `CompileWithOptions` example.

## XML and XSD behavior

Documents require one root element. Outside it, only literal XML whitespace,
comments, and processing instructions are allowed, with an optional XML
declaration at the start.

Empty complex content rejects all character data, including whitespace.
Element-only content allows XML whitespace around children, even when all
children are optional. Comments, processing instructions, and empty CDATA add
no characters, so they do not violate empty content.

Relative namespace names are accepted as an extension and compared exactly.
They do not change whether an attribute is a namespace declaration.

An empty element with a default or fixed value uses the schema's value. If
`xsi:type` changes the type, that value must satisfy the actual type's facets.
The value uses canonical spelling, except QName/NOTATION values retain their
schema spelling and namespace bindings. Instance prefix declarations cannot
change a schema-supplied QName. Explicit element text uses the instance's
namespaces and must equal any fixed value.

XSD regular expressions support class subtraction, `\i`/`\c`, and Unicode
category and block escapes. Compilation and matching have work limits.

## CLI and browser

From a repository checkout, validate local files with:

```sh
go run ./cmd/xmllint --schema schema.xsd document.xml
```

`--schema` is required. Optional flags are `--max-errors`,
`--max-identity-entries`, and `--max-instance-bytes`; zero selects their defaults
of 100, 100,000, and 64 MiB. The identity limit independently bounds stored
identity entries, pending selector matches, and pending field values.
Run `make xmllint` to build `bin/xmllint`.

To run the browser validator:

```sh
make web
```

Open [the local validator](http://127.0.0.1:8765). The command builds the Go WASM
module and serves the page. Validation runs in a Web Worker; clearing input
cancels active work.

Results are `valid`, `invalid`, or `error`. `invalid` means the document violates
the compiled schema. `error` means assessment could not complete, including
schema compilation failures, malformed XML, unsupported features, and resource
limits. Both unsuccessful results can include structured diagnostics. The page
formats XML only after a valid result.

Each worker reuses the compiled schema while the exact XSD text is unchanged.
Changing XSD replaces it; compilation failures leave the cache empty. Instance
errors retain the schema, and worker termination releases it.

Run JavaScript tests with `make web-test`. For browser integration tests, install
the pinned dependency and Chromium once:

```sh
npm ci --prefix docs/js
npm exec --prefix docs/js -- playwright install chromium
make browser-test
```

## Benchmarks

Go results were measured on 2026-10-20 using Go 1.27.0
on an Apple M2 Max with 32 GiB RAM. Libxml2 2.9.13
results are unchanged from the 2026-09-06 run on the same machine; libxml2 was
not rerun.

| Workload | Go time | libxml2 time | Go peak RSS | libxml2 peak RSS |
| --- | ---: | ---: | ---: | ---: |
| Streaming, 20 MiB | 499.797 ms | 377.093 ms | 6.594 MiB | 243.08 MiB |
| Streaming, 100 MiB | 2.422 s | 1.789 s | 6.766 MiB | 1.17 GiB |
| Streaming, 500 MiB | 11.978 s | 8.859 s | 6.938 MiB | 5.81 GiB |
| Streaming, 1 GiB | 24.435 s | 21.209 s | 6.891 MiB | 10.29 GiB |
| Streaming, 2 GiB | 48.796 s | 51.485 s | 6.812 MiB | 13.33 GiB |
| Identity constraints, 100,000 rows | 312.665 ms | 605.858 ms | 89.219 MiB | 186.66 MiB |

Each result is the upper median (sixth sorted value) of 10 runs after one
warm-up per workload and validator. Time and peak RSS are summarized
independently. Measurements include process startup and schema compilation.
Peak RSS is process memory reported by
`/usr/bin/time`, not cumulative Go allocations.

Go streams input; libxml2 uses tree mode with `--huge --noout --schema`.
The workloads are synthetic and do not compare libxml2's streaming mode.

See [benchmark methodology and reproduction](BENCHMARKS.md) for timing
variation, commands, and session-reuse guidance.

## Contributing

See [architecture](ARCHITECTURE.md) for design and ownership,
[test workflows](tests/README.md) for the corpus and harness, and the
[XSD 1.0 reference](docs/spec/README.md) for specification material.
