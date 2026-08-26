package xsd

import (
	"io"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

// SchemaSource identifies a schema document passed to Compile.
type SchemaSource struct {
	src source.Source
}

// Resolver resolves schema include/import locations during compilation.
type Resolver interface {
	ResolveSchema(base, location string) (SchemaSource, error)
}

// ResolverFunc adapts a function to Resolver.
type ResolverFunc func(base, location string) (SchemaSource, error)

// ResolveSchema resolves one schema include/import location.
func (f ResolverFunc) ResolveSchema(base, location string) (SchemaSource, error) {
	if f == nil {
		return SchemaSource{}, xsderrors.ErrSchemaNotFound
	}
	return f(base, location)
}

// File returns a file schema source and resolves local schemaLocation refs. A
// relative path is made absolute when File is called so its identity and
// include base remain stable if the process working directory changes.
func File(path string) SchemaSource {
	return SchemaSource{src: source.File(path)}
}

// Bytes returns an in-memory schema source from data.
func Bytes(name string, data []byte) SchemaSource {
	return SchemaSource{src: source.Bytes(name, data)}
}

// Open returns a reusable schema source backed by open. The function must
// return a new independent reader on every call.
func Open(name string, open func() (io.ReadCloser, error)) SchemaSource {
	return SchemaSource{src: source.Opener(name, open)}
}

// WithResolver returns s with r used for every schema include/import reached
// from s. A source returned by r remains in that resolver-owned graph, and its
// non-empty name is the authoritative document identity for deduplication and
// descendant resolution.
func (s SchemaSource) WithResolver(r Resolver) SchemaSource {
	s.src = s.src.WithResolver(adaptPublicResolver(r))
	return s
}

func internalSchemaSource(src SchemaSource) source.Source { return src.src }

func adaptPublicResolver(r Resolver) source.Resolver {
	if r == nil {
		return nil
	}
	return func(base, location string) (source.Source, error) {
		src, err := r.ResolveSchema(base, location)
		return src.src, err
	}
}
