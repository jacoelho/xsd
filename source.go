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
// Returning [xsderrors.ErrSchemaNotFound], wrapped or joined only with other
// misses, reports an unavailable location and permits normal fallback resolution.
// Any other failure, including one joined with a miss, stops compilation.
// A successful result must have a non-empty source name.
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
// return a new independent reader on every call. If open returns a non-nil
// error, compilation ignores the reader without closing it; open must release
// any resource it acquired before returning an error. Compilation reports the
// error, except that for an include/import target an error matching only
// [os.ErrNotExist] marks the location unavailable. Otherwise compilation owns and closes the returned reader and reports
// close errors; a nil reader is reported as a schema read error. The caller
// must not reuse a reader returned without an error.
func Open(name string, open func() (io.ReadCloser, error)) SchemaSource {
	return SchemaSource{src: source.Opener(name, open)}
}

// WithResolver returns s with r used for every schema include/import reached
// from s. A source returned by r remains in that resolver-owned graph regardless
// of its own resolver, and its non-empty name is the authoritative document
// identity for deduplication and descendant resolution. A nil resolver,
// including a nil ResolverFunc or *ResolverFunc, removes custom resolution
// from s; a File source keeps its built-in local schemaLocation resolution.
//
// Each call creates a distinct resolution context, even for the same r;
// resolvers are not compared. Copies of the result, and sources r returns,
// share its context. A document reached from several contexts is compiled once,
// keyed by its identity, but each context that reaches it resolves its
// references through its own resolver. Every time a document is reached again
// it is reopened and its bytes are charged against the compile limits again;
// different content under the same identity is a schema reference error.
func (s SchemaSource) WithResolver(r Resolver) SchemaSource {
	s.src = s.src.WithResolver(adaptPublicResolver(r))
	return s
}

func internalSchemaSource(src SchemaSource) source.Source { return src.src }

func adaptPublicResolver(r Resolver) source.Resolver {
	if r == nil {
		return nil
	}
	switch resolver := r.(type) {
	case ResolverFunc:
		if resolver == nil {
			return nil
		}
	case *ResolverFunc:
		if resolver == nil || *resolver == nil {
			return nil
		}
	}
	return func(base, location string) (source.Source, error) {
		src, err := r.ResolveSchema(base, location)
		return src.src, err
	}
}
