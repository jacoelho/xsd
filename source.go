package xsd

import (
	"bytes"
	"errors"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ErrSchemaNotFound reports that a resolver could not resolve a schema.
var ErrSchemaNotFound = errors.New("schema not found")

var errNilSchemaReader = errors.New("nil schema reader")

// SchemaSource identifies a schema document passed to Compile.
type SchemaSource struct {
	err      error
	resolver Resolver
	open     func() (io.ReadCloser, error)
	name     string
	data     []byte
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
		return SchemaSource{}, ErrSchemaNotFound
	}
	return f(base, location)
}

// File returns a file schema source and resolves local schemaLocation refs.
func File(path string) SchemaSource {
	path = filepath.Clean(path)
	return SchemaSource{
		name: path,
		open: func() (io.ReadCloser, error) {
			return os.Open(path)
		},
		resolver: ResolverFunc(resolveFileSchemaSource),
	}
}

// Reader reads r into an in-memory schema source.
func Reader(name string, r io.Reader) SchemaSource {
	if r == nil {
		return SchemaSource{name: name, err: errNilSchemaReader}
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return SchemaSource{name: name, err: err}
	}
	return SchemaSource{name: name, data: data}
}

// LimitedReader reads at most maxBytes from r into an in-memory schema source.
func LimitedReader(name string, r io.Reader, maxBytes int64) SchemaSource {
	if r == nil {
		return SchemaSource{name: name, err: errNilSchemaReader}
	}
	data, err := readLimitedSchemaSource(name, r, maxBytes)
	if err != nil {
		return SchemaSource{name: name, err: err}
	}
	return SchemaSource{name: name, data: data}
}

// WithResolver returns s with r used for schema include/import resolution.
func (s SchemaSource) WithResolver(r Resolver) SchemaSource {
	s.resolver = r
	return s
}

func (s SchemaSource) read(maxBytes int64) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.data != nil {
		if int64(len(s.data)) > maxBytes {
			return nil, schemaSourceLimitError(s.name)
		}
		return bytes.Clone(s.data), nil
	}
	if s.open == nil {
		return nil, schemaCompile(ErrSchemaRead, "schema source has no data or opener")
	}
	r, err := s.open()
	if err != nil {
		return nil, err
	}
	data, readErr := readLimitedSchemaSource(s.name, r, maxBytes)
	closeErr := r.Close()
	if readErr != nil {
		return nil, readErr
	}
	return data, closeErr
}

func readLimitedSchemaSource(name string, r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, schemaCompile(ErrSchemaLimit, "schema reader byte limit must be positive")
	}
	reader := r
	if maxBytes < math.MaxInt64 {
		reader = io.LimitReader(r, maxBytes+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, schemaSourceLimitError(name)
	}
	return data, nil
}

func schemaSourceLimitError(name string) error {
	if name == "" {
		return schemaCompile(ErrSchemaLimit, "schema source exceeds MaxSchemaSourceBytes")
	}
	return schemaCompile(ErrSchemaLimit, "schema source "+name+" exceeds MaxSchemaSourceBytes")
}

func resolveFileSchemaSource(base, location string) (SchemaSource, error) {
	path, ok := resolveLocalSchemaLocation(base, location)
	if !ok {
		return SchemaSource{}, ErrSchemaNotFound
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return SchemaSource{}, ErrSchemaNotFound
		}
		return SchemaSource{}, err
	}
	return File(path), nil
}

func resolveLocalSchemaLocation(base, location string) (string, bool) {
	u, err := url.Parse(location)
	if err == nil && u.Scheme != "" {
		if u.Scheme != "file" {
			return "", false
		}
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			return "", false
		}
		path, err := url.PathUnescape(u.Path)
		if err != nil || path == "" {
			return "", false
		}
		return filepath.Clean(path), true
	}
	location = filepath.FromSlash(strings.TrimSpace(location))
	if location == "" {
		return "", false
	}
	return filepath.Clean(filepath.Join(filepath.Dir(base), location)), true
}
