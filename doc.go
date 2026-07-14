// Package xsd compiles XML Schema 1.0 documents into an immutable Engine and
// validates XML instance documents with per-call streaming state.
//
// File resolves local xs:include and xs:import schemaLocation values relative
// to the referencing file. Bytes copies caller-owned schema bytes. Open defers
// repeatable context-aware stream acquisition until compilation, where compile
// limits apply.
// Compile uses only sources passed to it unless callers attach a Resolver with
// SchemaSource.WithResolver. HTTP and network fetches are never performed by
// default.
//
// Compile and validation operations take a context.Context for cooperative
// cancellation. Callbacks and readers that ignore cancellation cannot be
// forcibly interrupted.
//
// Instance validation is streaming. Engine.Validate consumes an io.Reader with
// a low-level byte XML parser, rejects DTD declarations, rejects non-UTF-8
// instance documents, and keeps mutable validation state inside the call.
package xsd
