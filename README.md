# xsd

A pure Go XML Schema 1.0 validator. Compile a schema once, then validate XML
from an `io.Reader`. Validation streams the document: it does not build a
document tree or store the full XML.

## What it supports

The library requires Go 1.27 or newer. It accepts XSD 1.0 and XML 1.0 in UTF-8.
The supported language is XSD 1.0 with approved errata and
[selected corrections](docs/spec/SOURCES.md#selected-corrections). XSD 1.1
syntax is not supported.

The library does not support DTDs, external entities, XML versions other than
1.0, or `xs:redefine`.

Schema sources are explicit. The library does not fetch schemas over HTTP or
any other network transport. Instance `xsi:schemaLocation` and
`xsi:noNamespaceSchemaLocation` hints never supply components. Validation uses
only the compiled component set and reports missing required declarations or
types as validation errors.

## Install and validate

In a Go module, run:

```sh
go get github.com/jacoelho/xsd
```

For a new program, run `go mod init example.com/validate` in an empty directory
first.

Save this as `main.go`. The schema declares a `root` element containing an
integer, and the XML supplies the value `7`.

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

Run `go run .` and it prints `valid`. Replace `7` with `hello` and validation
returns an error, because the text is not an integer.

`Compile` checks the schema. `Validate` checks one XML document against it. A
`nil` validation error means the document is valid.

The remaining Go examples are helper functions. Add them to your package with
the imports shown, merging any existing imports.

## Validate files

`xsd.File` returns a schema source for a local file. It resolves local includes and
imports relative to that file, respecting `xml:base` and local `file:` URIs. The
caller opens and closes the XML document:

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

## Handle errors

Compilation and validation return errors for schema violations. They also return
errors for unsupported features, malformed input and exceeded resource limits.
`xsderrors.Flatten` lists every diagnostic in an error:

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

A structured diagnostic is an `xsderrors.Error`, with methods for category,
code, message, path, line and column. `Message()` omits the cause and `Error()`
includes it. Several recoverable errors arrive as `xsderrors.Errors`, with
`Len()` and `At()` accessors.

Two predicates classify an error:

- `xsderrors.IsUnsupported(err)` reports an unsupported feature.
- `xsderrors.IsConclusiveValidation(err)` returns true only when every
  independent diagnostic branch establishes a schema violation. Malformed XML
  and resource failures return false. False can also mean there is not enough
  evidence to classify the error. It does not mean the XML is valid.

Validation messages include the constraint and the observed value:

```text
value "é" has length 1 character; expected exactly 2
value "1" does not satisfy minInclusive "2.0"
unexpected child element "c"; expected next child ["b"]
invalid attribute "{http://www.w3.org/1999/xlink}actuate": value "bogus" is not one of ["onLoad", "onRequest"]
```

Names include their namespace when needed. Enumeration choices use canonical
representations, and long values and lists are truncated with `...`. Identity
errors name the constraint and the field or referenced key. Detailed formatting
runs only after validation fails, including when a union has tried several
member types.

## Load schemas from memory, streams or a resolver

When the schemas are already in memory, pass them all to `Compile`.
`xsd.Bytes(name, data)` copies the schema bytes:

```go
import "github.com/jacoelho/xsd"

func compileSchemas(root, types []byte) (*xsd.Engine, error) {
	return xsd.Compile(
		xsd.Bytes("schema.xsd", root),
		xsd.Bytes("types.xsd", types),
	)
}
```

Here `root` can include `types.xsd` with
`<xs:include schemaLocation="types.xsd"/>`. Without a resolver, `Bytes` and
`Open` sources resolve references only from the sources passed to `Compile`.

Source names identify documents. Byte-identical copies under different names
are distinct documents, so two copies that declare the same components fail with
`schema.duplicate`. A resolver that returns one canonical name for equivalent
locations avoids this, for example for a bundled `xml.xsd`.

`xsd.Open(name, opener)` reads a schema from a stream. The opener must return a
new independent reader on every call. Compilation applies byte limits from the
first read.

When the opener returns an error, compilation does not close the reader, so the
opener must release anything it acquired first. Otherwise compilation closes the
returned reader and reports close errors, and the caller must not reuse it. The
`Open` doc comment covers the remaining error cases.

To supply include and import targets on demand, attach a resolver. This example
loads `types.xsd` from a map:

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

The resulting engine accepts `<root/>`. The map uses only `location`. A resolver
for nested paths should also use `base`.

Return `xsderrors.ErrSchemaNotFound` for a missing location to allow fallback
resolution. Wrapping a miss, or joining only misses, also allows fallback. Any
error that is not a miss stops compilation, including a miss joined with another
error. A successful result must have a non-empty source name. The resolver
attached to a root source handles every include and import reached from it. The `Resolver` and
`SchemaSource.WithResolver` doc comments define wrapped and joined misses,
source identity and repeated loads.

## Supply the XML and XLink schemas

The library has no built-in `xml.xsd` or `xlink.xsd`, and their namespace URIs
never trigger HTTP retrieval. Supply them through the same sources or resolver
as any other schema. An import without `schemaLocation` can use an explicitly
supplied second root, as in `compileSchemas` above.

The referring schema must import the namespace. The predefined `xml` prefix
does not replace that import, including for `xml:lang` and `xml:space`. The
supplied declarations control attribute types, facets and fixed or default
values. `xml:id` has ID semantics only when a supplied schema declares it with
an ID-derived type.

The XSD and XSI namespaces are built in and need no import. Supplied schemas
cannot replace their declarations. The four XSI attributes have fixed types:

| Attribute | Type |
| --- | --- |
| `xsi:type` | QName |
| `xsi:nil` | boolean |
| `xsi:noNamespaceSchemaLocation` | anyURI |
| `xsi:schemaLocation` | list of anyURI |

On assessed elements, each location hint item must be a valid anyURI, and
`MaxInstanceTokenBytes` bounds each hint attribute. XSI default or fixed values
do not supply missing attributes, so a required attribute must be present.

## Reuse an engine or session

An engine is safe to share across goroutines. Each validation call has its own
document state. The engine keeps at most one idle session for reuse, and
concurrent calls acquire separate state without waiting. Copies of an engine
share that cache.

For sequential validation with fixed options, create a session. A session
clears document state after each call but keeps bounded buffers and caches for
the next one:

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

A session is not safe for concurrent use. Copies of a session share its state,
and overlapping calls fail with `xsderrors.CodeValidationSession` before the
second call reads its input. Use `Engine.Validate` or separate sessions for
concurrent work. Discard a session to release its buffers.

## Set resource limits

Default limits bound compilation and validation. Set compilation limits with
`CompileWithOptions`. Set validation limits for one call with
`ValidateWithOptions`, or for a session with `NewSession`.

Compilation and validation are synchronous. Limits cannot interrupt a blocked
opener, resolver, file operation or reader. If you need interruption, provide
I/O that you can close or otherwise unblock.

[Resource limits](docs/RESOURCE_LIMITS.md) lists every limit, its default and
its zero-value rule.

## XML and XSD behavior

A document requires one root element. Outside it, only literal XML whitespace,
comments and processing instructions are allowed, with an optional XML declaration at the
start.

Empty complex content rejects all character data, including whitespace.
Element-only content allows XML whitespace around children, even when all
children are optional. Comments, processing instructions and empty CDATA
sections add no characters, so they do not violate empty content.

An empty element with a default or fixed value takes the schema's value. If
`xsi:type` changes the type, that value must satisfy the actual type's facets.
The value uses canonical spelling, except that QName and NOTATION values keep
their schema spelling and namespace bindings. Instance prefix declarations
cannot change a schema-supplied QName. Explicit element text uses the
instance's namespaces and must equal any fixed value.

Relative namespace names are accepted as an extension and compared exactly.
They do not change whether an attribute is a namespace declaration.

XSD regular expressions support class subtraction, `\i` and `\c`, and Unicode
category and block escapes. Compiling and matching patterns have work limits.

## Validate from the command line

From a repository checkout, validate a local file with:

```sh
go run ./cmd/xmllint --schema schema.xsd document.xml
```

`--schema` is required. The optional flags `--max-errors`,
`--max-identity-entries` and `--max-instance-bytes` set validation limits. Zero
selects the library default from [resource limits](docs/RESOURCE_LIMITS.md). Run `make xmllint` to build `bin/xmllint`.

## Validate in a browser

To build the Go WebAssembly module and serve the validator page, run:

```sh
make web
```

Then open <http://127.0.0.1:8765>. Validation runs in a Web Worker, and clearing
the input cancels active work.

A result is `valid`, `invalid` or `error`:

- `invalid` means the document violates the compiled schema.
- `error` means assessment could not complete, for example because the schema
  failed to compile, the XML is malformed, a feature is unsupported or a
  resource limit was reached.

Both unsuccessful results can include structured diagnostics. The page formats
the XML only after a valid result.

Each worker reuses the compiled schema while the XSD text is unchanged.
Changing the XSD replaces it, and a compilation failure leaves no cached schema.
Instance errors keep the schema, and terminating the worker releases it.

Run the JavaScript tests with `make web-test`. The browser integration tests
need the pinned dependencies and Chromium, installed once:

```sh
npm ci --prefix web/js
npm exec --prefix web/js -- playwright install chromium
make browser-test
```

## Benchmarks

These results compare this library with libxml2's `xmllint --stream`, measured
with `make bench-libxml2` on 2026-11-15. The machine was an Apple M2 Max with
32 GiB RAM, running Go 1.27.3 and libxml2 2.9.13. Both validators used the same
generated files and ran alternately.

CPU time is user plus system time. All measurements include process startup and
schema compilation. Tables before 2026-11-15 timed libxml2 with
`/usr/bin/time` and `bin/xmllint` around that wrapper, so their elapsed times are
not directly comparable.

| Workload | Go CPU | libxml2 `--stream` CPU | CPU ratio | Go elapsed | libxml2 elapsed | Go peak RSS | libxml2 peak RSS |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Streaming, 20 MiB | 351.792 ms | 304.398 ms | 1.156 | 352.819 ms | 305.499 ms | 6.59 MiB | 1.91 MiB |
| Streaming, 100 MiB | 1.734 s | 1.487 s | 1.166 | 1.734 s | 1.488 s | 6.58 MiB | 1.91 MiB |
| Streaming, 500 MiB | 8.604 s | 7.367 s | 1.168 | 8.601 s | 7.369 s | 6.77 MiB | 1.91 MiB |
| Streaming, 1 GiB | 17.590 s | 15.075 s | 1.167 | 17.584 s | 15.078 s | 6.83 MiB | 1.91 MiB |
| Streaming, 2 GiB | 35.095 s | 30.106 s | 1.166 | 35.081 s | 30.110 s | 6.94 MiB | 1.91 MiB |
| Identity constraints, 100,000 rows | 302.574 ms | 300.383 ms | 1.007 | 265.724 ms | 301.699 ms | 75.33 MiB | 74.66 MiB |

[Benchmark methodology](docs/BENCHMARKS.md) covers measurement rules,
reproduction commands and session-reuse guidance.

## Contributing

- [Architecture](ARCHITECTURE.md) covers design and ownership.
- [Test workflows](tests/README.md) covers the conformance corpus and harness.
- The [XSD 1.0 reference](docs/spec/README.md) holds the specification
  material.
