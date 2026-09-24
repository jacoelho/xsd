# xsd

Pure Go XML Schema 1.0 validator. Compile a schema once, then validate XML from
an `io.Reader` without building a DOM or storing the full document.

Requires Go 1.27 or newer.

## Install

In your Go module:

```sh
go get github.com/jacoelho/xsd
```

## Quickstart

Save this as `main.go` and run `go run .`:

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

The remaining Go examples are functions you can add below `main`. Each section
notes any additional imports; function arguments supply the input data.

## Supported inputs

- XSD 1.0 and XML 1.0, encoded as UTF-8. XML 1.1 is rejected.
- DTDs, external entities, and `xs:redefine` are unsupported.
- XSD regular expressions include class subtraction, `\i`/`\c`, and Unicode
  category and block escapes. Pattern compilation and matching have work limits.
- Schema sources are explicit. The library does not fetch schemas over the
  network, and instance `xsi:schemaLocation` hints never trigger loading.

XML documents require one root element. Apart from an optional XML declaration
at the start, only literal XML whitespace, comments, and processing instructions
may appear outside that element.

An empty element with a default or fixed value uses the schema's value. When
`xsi:type` changes its type, that value must satisfy the actual type's facets.
Application uses canonical spelling, except QName/NOTATION values retain their
source spelling and schema namespace bindings. Instance prefix declarations
cannot change a schema-supplied QName. Explicit element text uses the instance's
namespaces and remains subject to fixed-value equality.

## Schema sources

Pass one or more sources to `xsd.Compile`:

| Source | Use |
| --- | --- |
| `xsd.Bytes(name, data)` | In-memory schema bytes; copies the supplied data. |
| `xsd.File(path)` | Local schema files; resolves local include/import locations relative to each file, including `xml:base` and local `file:` URIs. |
| `xsd.Open(name, opener)` | Repeatable streams; compilation applies byte limits from the first read. |

`Bytes` and `Open` resolve references only from explicitly supplied sources
unless you attach a resolver. Source names identify documents; resolver-returned
sources must have non-empty names.

### Validate a file

Add `os` to the imports. Call this with your schema and XML file paths:

```go
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

### Compile a stream

Add `io` to the imports. The opener must return a new independent reader on
every call. Compilation owns and closes every non-nil reader it returns, even
when the opener also returns an error; close errors are reported.

```go
func compileStream(schema string) (*xsd.Engine, error) {
	return xsd.Compile(xsd.Open("schema.xsd", func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(schema)), nil
	}))
}
```

### Resolve includes and imports

Add `"github.com/jacoelho/xsd/xsderrors"` to the imports. This example resolves an
include from an in-memory map:

```go
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

The resulting engine accepts `<root/>`. This flat map uses only `location`;
a resolver for nested paths should also use the supplied `base`.

Return `xsderrors.ErrSchemaNotFound` for an unavailable location, allowing normal
fallback resolution. Wrapping a miss or joining only misses also permits fallback.
Any other failure, including one joined with a miss, stops compilation. A
successful result is authoritative. The resolver applies to all descendants;
a returned source's own resolver is ignored.

## Validation and reuse

`Engine.Validate` accepts an `io.Reader` and returns `nil` for valid XML.
An engine is immutable and safe to share across goroutines; each call owns
isolated document state.

For repeated sequential validation, a session reuses bounded buffers and caches:

```go
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

A session clears document state after each call. Discard it to release retained
buffers and caches. Copies share the same state: overlapping calls fail with
`xsderrors.CodeValidationSession` before consuming the second input. Use
`Engine.Validate` or separately constructed sessions for concurrent work.

### Inspect errors

Add `errors` and `"github.com/jacoelho/xsd/xsderrors"` to the imports. Pass the
error returned by compilation or validation to this function:

```go
func printDiagnostics(err error) {
	if group, ok := errors.AsType[xsderrors.Errors](err); ok {
		for i := range group.Len() {
			fmt.Println(group.At(i))
		}
		return
	}
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok {
		fmt.Printf("%s [%s] %s:%d:%d: %s\n",
			diagnostic.Category(), diagnostic.Code(), diagnostic.Path(),
			diagnostic.Line(), diagnostic.Column(), diagnostic.Message())
	} else if err != nil {
		fmt.Println(err)
	}
}
```

`xsderrors.Error` exposes category, code, message, path, line, and column.
Multiple recoverable errors are returned as `xsderrors.Errors`; use `Len` and
`At` to inspect them all. Categories are `schema_parse`, `schema_compile`,
`unsupported`, `validation`, `format`, and `internal`.
Use `xsderrors.IsUnsupported(err)` to detect unsupported features.

## Resource limits

Defaults bound schema compilation and document validation. Set only the limits
you need to change. Zero selects the default except where stated below;
negative signed limits are errors.

### Compilation

Use `CompileWithOptions` instead of `Compile`:

```go
func compileWithLimits(schema []byte) (*xsd.Engine, error) {
	return xsd.CompileWithOptions(
		xsd.CompileOptions{MaxSchemaSourceBytes: 8 << 20, MaxSchemaSources: 32},
		xsd.Bytes("schema.xsd", schema),
	)
}
```

| Option | Default | Meaning |
| --- | ---: | --- |
| `MaxSchemaDepth` | `256` | Max nested schema XML elements. |
| `MaxSchemaAttributes` | `256` | Max attributes on one schema XML element. |
| `MaxSchemaTokenBytes` | `4 MiB` | Max retained schema XML token payload. |
| `MaxSchemaSourceBytes` | `64 MiB` | Max bytes read from each schema source. |
| `MaxSchemaSources` | `1024` | Max explicit source descriptors and distinct resolver-loaded source identities admitted to one compilation. |
| `MaxSchemaTotalBytes` | `256 MiB` | Max aggregate bytes read across all schema sources. |
| `MaxSchemaReferences` | `16_384` | Max include/import references processed across the schema set. |
| `MaxSchemaDependencySteps` | `1_000_000` | Max aggregate schema-graph expansion, target-context propagation, and component-dependency resolution work. |
| `MaxSchemaTargetContexts` | `4096` | Max distinct source/effective-target-namespace contexts, including primary and chameleon-derived contexts. |
| `MaxSchemaInstantiatedNodes` | `1_000_000` | Max aggregate schema node occurrences across effective target contexts. |
| `MaxSchemaNames` | `0` | Max interned schema names, including built-ins. `0` means no explicit limit. |
| `MaxFiniteOccurs` | `0` | Max accepted finite `maxOccurs`. `0` uses the runtime `uint32` cap. |
| `MaxContentModelStates` | `16_384` | Max DFA states per compiled content model. |
| `MaxContentModelAnalysisSteps` | `16_777_216` | Max content-model traversal, determinization, and ambiguity-analysis work. |
| `MaxSubstitutionClosureEntries` | `1_000_000` | Max aggregate transitive substitution-group relationships. |
| `MaxSimpleUnionMemberEntries` | `1_000_000` | Max aggregate flattened simple-union members. |

These limits apply to explicit sources and resolver-loaded includes/imports.
Finite `minOccurs` and `maxOccurs` values cannot exceed `4294967295`.
`MaxFiniteOccurs` can lower the finite `maxOccurs` cap;
`maxOccurs="unbounded"` is unaffected.

### Validation

Use `ValidateWithOptions` for one call, or pass the same options to `NewSession`.
Add `io` to the imports for this example:

```go
func validateWithLimits(engine *xsd.Engine, doc io.Reader) error {
	return engine.ValidateWithOptions(doc, xsd.ValidateOptions{
		MaxErrors:        1,
		MaxInstanceBytes: 8 << 20,
	})
}
```

| Option | Default | Meaning |
| --- | ---: | --- |
| `MaxErrors` | `100` | Max collected recoverable validation errors. |
| `MaxIdentityScopes` | `10_000` | Max active identity-constraint scopes. |
| `MaxIdentityEntries` | `100_000` | Independent max for stored ID, IDREF, key, unique, and keyref entries, pending identity-selector matches, and pending identity-field values. |
| `MaxIdentityTupleBytes` | `4 KiB` | Max byte length of one stored identity key. |
| `MaxSchemaLocationNamespaces` | `256` | Max distinct schema-location namespace names retained per document. |
| `MaxSchemaLocationNamespaceBytes` | `64 KiB` | Max aggregate bytes in distinct retained schema-location namespace names. `MaxInstanceTokenBytes` also bounds each complete hint attribute. |
| `MaxInstanceDepth` | `256` | Max nested XML elements. |
| `MaxInstanceAttributes` | `4,096` | Max attributes on one XML element. |
| `MaxInstanceTextBytes` | `4 MiB` | Max retained character data bytes. |
| `MaxInstanceTokenBytes` | `4 MiB` | Max parser-owned bytes for one XML token, including retained payload and active construction scratch. |
| `MaxInstanceBytes` | `64 MiB` | Max aggregate raw XML bytes read, including a UTF-8 BOM and XML declaration. |
| `MaxInstanceValueWork` | `4_194_502_132_335` | Max cumulative lexical work per simple-value evaluation. Independent of schema compilation limits. |

Each value-evaluation visit charges its lexical byte length plus one. List items,
union attempts, and a raw-byte attempt followed by typed fallback share the
containing value's budget. Each new value starts a fresh budget. URI items in
`xsi:schemaLocation` hints are separate typed evaluations. The work limit also
applies to schema defaults or fixed values that require revalidation; already
validated defaults keep their existing path. Regex and byte limits apply
separately. Zero selects the default; positive values set a finite work ceiling.

Reaching `MaxErrors` stops semantic assessment, but XML is still read until the
end or a fatal error. A later fatal error takes precedence.

### I/O interruption

Compilation and validation are synchronous. Limits bound admitted input and
work; they cannot interrupt a blocked opener, resolver, file operation, or
reader. Callers that need interruption must provide I/O they can close or
otherwise unblock.

## CLI

From a repository checkout, validate local files with:

```sh
go run ./cmd/xmllint --schema schema.xsd document.xml
```

| Flag | Required | Meaning |
| --- | --- | --- |
| `--schema path` | yes | Schema file path. |
| `--max-errors n` | no | Maximum collected validation errors; `0` selects 100. |
| `--max-identity-entries n` | no | Independent maximum for stored identity entries, pending selector matches, and pending field values; `0` selects 100,000. |
| `--max-instance-bytes n` | no | Maximum raw XML bytes read; `0` selects 64 MiB. |

Run `make xmllint` to build `bin/xmllint`.

## Browser validator

From a repository checkout:

```sh
make web
```

Open [the local validator](http://127.0.0.1:8765). This command builds the Go WASM
module and serves the page. Validation runs in a Web Worker; clearing input
cancels active work.

Run JavaScript tests with `make web-test`. To run browser integration tests,
install the pinned dependency and Chromium once:

```sh
npm ci --prefix docs/js
npm exec --prefix docs/js -- playwright install chromium
make browser-test
```

## Benchmarks

Comparison of `main` at `175722ab` with libxml2 2.9.13, measured on
2026-09-06 using Go 1.27.0, macOS 26.6.2, and an Apple M2 Max with 32 GiB RAM.
Each result is the upper median (sixth sorted value) of 10 runs after one
warm-up per workload and validator. Time and peak RSS are summarized
independently. The validators ran sequentially on the same generated files.

| Workload | `main` time | libxml2 time | `main` peak RSS | libxml2 peak RSS |
| --- | ---: | ---: | ---: | ---: |
| Streaming, 20 MiB | 686.685 ms | 377.093 ms | 8.11 MiB | 243.08 MiB |
| Streaming, 100 MiB | 3.336 s | 1.789 s | 11.94 MiB | 1.17 GiB |
| Streaming, 500 MiB | 16.453 s | 8.859 s | 13.38 MiB | 5.81 GiB |
| Streaming, 1 GiB | 33.486 s | 21.209 s | 13.72 MiB | 10.29 GiB |
| Streaming, 2 GiB | 67.063 s | 51.485 s | 14.30 MiB | 13.33 GiB |
| Identity constraints, 100,000 rows | 348.766 ms | 605.858 ms | 88.31 MiB | 186.66 MiB |

Libxml2 timings varied on the larger files: 20.86–37.68 s for 1 GiB and
51.08–52.20 s for 2 GiB across the 10 samples.

Timings include process startup, schema compilation, and validation. Go streams
input; libxml2 runs in tree mode with `--huge --noout --schema`. Peak RSS is
process memory reported by `/usr/bin/time`, not cumulative Go allocations.
These synthetic workloads are not a production traffic mix or a comparison of
libxml2's streaming mode.

The [benchmark harness](tests/large_benchmark_test.go) generates documents from
20 MiB through 2 GiB, plus 100,000 rows with ID/IDREF, key, unique, and keyref
constraints. It runs the repository's `bin/xmllint` and sets input-byte and
identity-entry limits to admit each generated document.

To reproduce the Go measurements and retain the inputs for libxml2:

```sh
make xmllint
benchmark_dir=$(mktemp -d)
XSD_LARGE_BENCHMARK=1 XSD_LARGE_RUNS=10 XSD_LARGE_DIR="$benchmark_dir" \
  go test ./tests -run '^TestLargeXMLLintBenchmark$' -timeout=45m -count=1 -v
```

Then run libxml2 on those same inputs. Select its executable explicitly so it
cannot be confused with the repository's Go binary. This command uses macOS
`/usr/bin/time -l`; use `-v` on Linux. Discard the warm-up, then sort
the 10 elapsed times and 10 peak-RSS measurements separately and take the sixth
value from each to match the Go harness.

```sh
libxml2=/usr/bin/xmllint
"$libxml2" --version
for xml in "$benchmark_dir"/streaming/*/document.xml "$benchmark_dir"/identity/document.xml; do
  case "$xml" in
    */streaming/*) schema="$benchmark_dir/streaming/schema.xsd" ;;
    *) schema="$benchmark_dir/identity/schema.xsd" ;;
  esac
  "$libxml2" --huge --noout --schema "$schema" "$xml" # warm-up
  for sample in 1 2 3 4 5 6 7 8 9 10; do
    /usr/bin/time -l "$libxml2" --huge --noout --schema "$schema" "$xml"
  done
done
```

For a smoke run, add
`XSD_LARGE_SIZE_BYTES=1048576 XSD_LARGE_IDENTITY_ROWS=1000 XSD_LARGE_RUNS=1`
to the Go command. Without `XSD_LARGE_DIR`, the harness removes generated
files; with it, remove the retained directory when finished.

## Contributing

See [ARCHITECTURE.md](ARCHITECTURE.md) for design and ownership,
[tests/README.md](tests/README.md) for test and corpus workflows, and
[docs/spec](docs/spec/README.md) for the XSD 1.0 reference.
