// Package preprocessor loads schema documents, resolves include/import
// directives, and applies deterministic schema merges during normalization.
// The root package owns loader/session state, directive control flow,
// deferred-resolution state, source-document ingestion, and merge mechanics.
package preprocessor
