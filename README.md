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

Return exactly `xsderrors.ErrSchemaNotFound` for an unavailable location, allowing
normal fallback resolution. Any other error, including a wrapped or joined
sentinel, stops compilation. A successful result is authoritative. The resolver
applies to all descendants; a returned source's own resolver is ignored.

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

## Library benchmarks

Measured 2026-09-06: rewrite `bb9045cd` versus `main` at `cc94656a`, using
Go 1.27.0 on macOS/arm64 (Apple M2 Max), one CPU, and six alternating 200 ms
samples per workload. Across **82 matching public workloads**, the geometric
mean of workload median times is **27.74% lower**. Workloads are equally
weighted; this is not an assumed production traffic mix.

| Workload | Main | Rewrite | Time change |
| --- | ---: | ---: | ---: |
| Small-schema compilation | 200.3 µs | 105.9 µs | -47.14% |
| Deep type-chain compilation | 8.57 ms | 4.28 ms | -50.04% |
| Repeated QName validation | 169.9 µs | 76.7 µs | -54.86% |
| Regex-category compilation | 2.95 ms | 1.87 ms | -36.63% |
| 16 MiB streamed schema compilation | 229.1 ms | 95.1 ms | -58.49% |
| Identity validation, 1,000 rows | 2.84 ms | 2.62 ms | -7.59% |
| Fixed gDay validation, 128 values | 261.9 µs | 185.1 µs | -29.33% |
| Repeated small-document session | 280.0 µs | 278.6 µs | Within noise |
| Substitution-group compilation | 692.2 µs | 699.9 µs | +1.12% |

Memory results vary by workload. A 16 MiB streamed schema allocates
**34.7 MiB → 122.8 KiB**. The 1,000-row and depth-256 identity cases allocate
8.3% and 17.2% more bytes, respectively. Fixed-value checks allocate more to
preserve typed equality. These are cumulative
Go allocations per operation, not peak memory or RSS. Schema-text compilation
has a 5.22% slower median, but the timing difference is within noise.

## Large XML benchmark

Build the repository's Go `xmllint` binary into `bin`:

```sh
make xmllint
```

The benchmark executes only `bin/xmllint`. It does not inspect `PATH`, require libxml2, or run another validator.

Run the full library-only benchmark with five measured samples per profile:

```sh
XSD_LARGE_BENCHMARK=1 XSD_LARGE_RUNS=5 go test ./tests -run TestLargeXMLLintBenchmark -timeout=0 -v
```

By default this generates streaming XML documents at `20MB`, `100MB`, `500MB`, `1GB`, and `2GB`, plus an identity-constraint document. The harness passes each generated file size through `--max-instance-bytes`, overriding the CLI's finite default. `XSD_LARGE_RUNS` controls measured samples per profile; results use the median below 20 samples and nearest-rank p95 at 20 or more. Generated files use `t.TempDir()` and are removed after each subtest. Set `XSD_LARGE_DIR=/path/to/dir` to keep generated files. Set `XSD_LARGE_SIZE_BYTES=1048576 XSD_LARGE_RUNS=1` for a quick single-size smoke run.

The benchmark reports elapsed time and max RSS from `/usr/bin/time` (`-l` on Darwin, `-v` on Linux). Max RSS is process memory, not Go `allocs/op`.

### Historical libxml2 comparison

The following local run is retained as historical context only. The current benchmark does not rerun or require libxml2.

Historical local run (2026-06-17, macOS 26.5, Go 1.26.4, libxml2 2.9.13, `main`, p95 over 20 runs):

```text
goos: darwin
goarch: arm64
pkg: github.com/jacoelho/xsd

                         | libxml2 xmllint |             go xmllint             |
                         | p95 sec/op      | p95 sec/op      vs base           |
streaming/20MB                 400.405ms       348.812ms      -12.89%
streaming/100MB                   1.792s          1.685s       -5.98%
streaming/500MB                  13.447s          8.336s      -38.01%
streaming/1GB                    26.104s         16.685s      -36.08%
streaming/2GB                    52.113s         33.715s      -35.30%
identity                       619.045ms       212.698ms      -65.64%
geomean                           4.484s          2.893s      -35.48%

                         | libxml2 xmllint |             go xmllint             |
                         | p95 rss/op      | p95 rss/op      vs base           |
streaming/20MB                 243.19MiB         6.97MiB      -97.13%
streaming/100MB                  1.17GiB         7.03MiB      -99.41%
streaming/500MB                  5.63GiB         7.34MiB      -99.87%
streaming/1GB                    8.45GiB         7.33MiB      -99.92%
streaming/2GB                   12.32GiB         7.53MiB      -99.94%
identity                       188.55MiB        69.73MiB      -63.01%
geomean                          1.76GiB        10.56MiB      -99.42%
```

## Contributing

See [ARCHITECTURE.md](ARCHITECTURE.md) for design and ownership,
[tests/README.md](tests/README.md) for test and corpus workflows, and
[docs/spec](docs/spec/README.md) for the XSD 1.0 reference.
