// Package preprocessor loads schema documents and resolves include/import directives.
// The root package keeps only loader/session state and orchestration; directive
// control flow, deferred-resolution state, source-document ingestion,
// deterministic merge mechanics, and schema-location resolution live in sibling
// subpackages.
package preprocessor
