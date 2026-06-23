package xsd

import (
	"io"
	"math"

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

// File returns a file schema source and resolves local schemaLocation refs.
func File(path string) SchemaSource {
	return SchemaSource{src: source.File(path)}
}

// Bytes returns an in-memory schema source from data.
func Bytes(name string, data []byte) SchemaSource {
	return SchemaSource{src: source.Bytes(name, data)}
}

// Reader reads r into an in-memory schema source.
func Reader(name string, r io.Reader) SchemaSource {
	return LimitedReader(name, r, math.MaxInt64)
}

// LimitedReader reads at most maxBytes from r into an in-memory schema source.
func LimitedReader(name string, r io.Reader, maxBytes int64) SchemaSource {
	return SchemaSource{src: source.LimitedReader(name, r, maxBytes)}
}

// WithResolver returns s with r used for schema include/import resolution.
func (s SchemaSource) WithResolver(r Resolver) SchemaSource {
	s.src = s.src.WithResolver(adaptPublicResolver(r))
	return s
}

func internalSchemaSources(sources []SchemaSource, scratch []source.Source) []source.Source {
	var out []source.Source
	if len(sources) <= cap(scratch) {
		out = scratch[:len(sources)]
	} else {
		out = make([]source.Source, len(sources))
	}
	for i, src := range sources {
		out[i] = src.src
	}
	return out
}

func adaptPublicResolver(r Resolver) source.Resolver {
	if r == nil {
		return nil
	}
	return func(base, location string) (source.Source, error) {
		src, err := r.ResolveSchema(base, location)
		return src.src, err
	}
}
