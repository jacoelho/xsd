package resolve

import "io"

// Kind identifies the kind of schema resolution request.
type Kind uint8

const (
	Include Kind = iota
	Import
)

// Request describes a schema resolution request.
type Request struct {
	BaseSystemID   string
	SchemaLocation string
	ImportNS       []byte
	Kind           Kind
}

// Resolver resolves schema documents into readers and canonical system IDs.
type Resolver interface {
	Resolve(req Request) (doc io.ReadCloser, systemID string, err error)
}
