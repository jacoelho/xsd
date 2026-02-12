package preprocessor

import "io"

// ResolveKind identifies the kind of schema resolution request.
type ResolveKind uint8

const (
	// ResolveInclude is an exported constant.
	ResolveInclude ResolveKind = iota
	// ResolveImport is an exported constant.
	ResolveImport
)

// ResolveRequest describes a schema resolution request.
type ResolveRequest struct {
	BaseSystemID   string
	SchemaLocation string
	ImportNS       []byte
	Kind           ResolveKind
}

// Resolver resolves schema documents into readers and canonical system IDs.
type Resolver interface {
	Resolve(req ResolveRequest) (doc io.ReadCloser, systemID string, err error)
}
