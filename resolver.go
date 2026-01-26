package xsd

import "github.com/jacoelho/xsd/internal/loader"

// ResolveKind identifies the kind of schema resolution request.
type ResolveKind = loader.ResolveKind

const (
	ResolveInclude ResolveKind = loader.ResolveInclude
	ResolveImport  ResolveKind = loader.ResolveImport
)

// ResolveRequest describes a schema resolution request.
type ResolveRequest = loader.ResolveRequest

// Resolver resolves schema documents into readers and canonical system IDs.
type Resolver = loader.Resolver
