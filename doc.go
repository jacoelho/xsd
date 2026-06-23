// Package xsd compiles XML Schema 1.0 documents into an immutable Engine and
// validates XML instance documents with per-call streaming state.
//
// File resolves local xs:include and xs:import schemaLocation values relative
// to the referencing file. Bytes copies caller-owned schema bytes. Reader
// eagerly reads trusted in-memory schemas, and LimitedReader caps untrusted
// reader input. Compile uses only sources passed to it unless callers attach a
// Resolver with SchemaSource.WithResolver. HTTP and network fetches are never
// performed by default.
//
// Instance validation is streaming. Engine.Validate consumes an io.Reader with
// a low-level byte XML parser, rejects DTD declarations, rejects non-UTF-8
// instance documents, and keeps mutable validation state inside the call.
package xsd
