# Resource limits

[Return to the README](README.md#resource-limits).

Defaults bound schema compilation and document validation. Set only the limits
you need to change. Zero selects the default except where stated below;
negative signed limits are errors.

## Compilation

Use `CompileWithOptions` instead of `Compile`. This helper requires the
`github.com/jacoelho/xsd` import:

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

## Validation

Use `ValidateWithOptions` for one call, or pass the same options to `NewSession`.
This helper requires `io` and `github.com/jacoelho/xsd`:

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

## I/O interruption

Compilation and validation are synchronous. Limits bound admitted input and
work; they cannot interrupt a blocked opener, resolver, file operation, or
reader. Callers that need interruption must provide I/O they can close or
otherwise unblock.
