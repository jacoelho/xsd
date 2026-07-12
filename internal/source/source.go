// Package source defines internal schema source and resolver primitives.
package source

import (
	"bytes"
	"errors"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/xsderrors"
)

// ErrNilReader reports a nil schema reader.
var ErrNilReader = errors.New("nil schema reader")

// Source identifies a schema document passed to compilation.
type Source struct {
	err      error
	resolver Resolver
	open     func() (io.ReadCloser, error)
	name     string
	data     []byte
}

// Resolver resolves schema include/import locations during compilation.
type Resolver func(base, location string) (Source, error)

// ResolveSchema resolves one schema include/import location.
func (r Resolver) ResolveSchema(base, location string) (Source, error) {
	if r == nil {
		return Source{}, xsderrors.ErrSchemaNotFound
	}
	return r(base, location)
}

// File returns a file schema source and resolves local schemaLocation refs.
func File(file string) Source {
	file = filepath.Clean(file)
	return Source{
		name: file,
		open: func() (io.ReadCloser, error) {
			return os.Open(file)
		},
		resolver: Resolver(resolveFileSchemaSource),
	}
}

// LimitedReader reads at most maxBytes from r into an in-memory schema source.
func LimitedReader(name string, r io.Reader, maxBytes int64) Source {
	if r == nil {
		return Source{name: name, err: ErrNilReader}
	}
	data, _, err := readLimitedSchemaSource(name, r, maxBytes)
	if err != nil {
		return Source{name: name, err: err}
	}
	return Source{name: name, data: data}
}

// Bytes returns an in-memory schema source.
func Bytes(name string, data []byte) Source {
	if data == nil {
		data = []byte{}
	}
	return Source{name: name, data: bytes.Clone(data)}
}

// Opener returns a schema source backed by an opener.
func Opener(name string, open func() (io.ReadCloser, error)) Source {
	return Source{name: name, open: open}
}

// WithResolver returns s with r used for schema include/import resolution.
func (s Source) WithResolver(r Resolver) Source {
	s.resolver = r
	return s
}

// Name returns the source name.
func (s Source) Name() string {
	return s.name
}

// Resolve resolves location relative to s through its attached resolver. A
// missing resolver or ErrSchemaNotFound reports found=false. Returned sources
// inherit the parent resolver when they do not provide one.
func (s Source) Resolve(location string) (resolved Source, found bool, err error) {
	if s.resolver == nil {
		return Source{}, false, nil
	}
	resolved, err = s.resolver.ResolveSchema(s.name, location)
	if errors.Is(err, xsderrors.ErrSchemaNotFound) {
		return Source{}, false, nil
	}
	if err != nil {
		return Source{}, false, err
	}
	if resolved.resolver == nil {
		resolved.resolver = s.resolver
	}
	return resolved, true, nil
}

// Read returns a copy of the source bytes.
func (s Source) Read(maxBytes int64) ([]byte, error) {
	data, _, err := s.ReadWithLimit(maxBytes)
	return data, err
}

// ReadWithLimit returns a copy of the source bytes and reports whether this
// read exceeded maxBytes. A limit error captured while constructing the Source
// is returned without setting limitExceeded.
func (s Source) ReadWithLimit(maxBytes int64) (data []byte, limitExceeded bool, err error) {
	if s.err != nil {
		return nil, false, s.err
	}
	if s.data != nil {
		if int64(len(s.data)) > maxBytes {
			return nil, true, schemaSourceLimitError(s.name)
		}
		return bytes.Clone(s.data), false, nil
	}
	if s.open == nil {
		return nil, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source has no data or opener")
	}
	r, err := s.open()
	if err != nil {
		return nil, false, err
	}
	data, limitExceeded, readErr := readLimitedSchemaSource(s.name, r, maxBytes)
	closeErr := r.Close()
	if readErr != nil {
		return data, limitExceeded, readErr
	}
	return data, false, closeErr
}

func readLimitedSchemaSource(name string, r io.Reader, maxBytes int64) ([]byte, bool, error) {
	if maxBytes < 0 {
		return nil, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema reader byte limit cannot be negative")
	}
	reader := r
	if maxBytes < math.MaxInt64 {
		reader = io.LimitReader(r, maxBytes+1)
	}
	data, err := io.ReadAll(reader)
	if int64(len(data)) > maxBytes {
		return data, true, schemaSourceLimitError(name)
	}
	if err != nil {
		return data, false, err
	}
	return data, false, nil
}

func schemaSourceLimitError(name string) error {
	msg := "schema source exceeds MaxSchemaSourceBytes"
	if name != "" {
		msg = "schema source " + name + " exceeds MaxSchemaSourceBytes"
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, msg)
}

// IsSchemaLimitError reports whether err is a schema source byte-limit diagnostic.
func IsSchemaLimitError(err error) bool {
	x, ok := errors.AsType[*xsderrors.Error](err)
	return ok && x.Code == xsderrors.CodeSchemaLimit
}

func resolveFileSchemaSource(base, location string) (Source, error) {
	file, ok := ResolveLocalSchemaLocation(base, location)
	if !ok {
		return Source{}, xsderrors.ErrSchemaNotFound
	}
	return File(file), nil
}

// Key canonicalizes a schema source name for loaded-document identity.
func Key(name string) string {
	if filepath.VolumeName(name) != "" {
		return filepath.Clean(name)
	}
	u, err := url.Parse(name)
	if err == nil && u.Scheme != "" {
		if file, ok := LocalFileURIPath(u); ok {
			return file
		}
		if u.Opaque != "" {
			return name
		}
		if u.Host != "" || u.Path != "" {
			if u.Path != "" {
				u.Path = path.Clean(u.Path)
				if u.Path == "." {
					u.Path = ""
				}
			}
			return u.String()
		}
	}
	return filepath.Clean(name)
}

// LocationKeys returns possible loaded-source keys for schemaLocation.
func LocationKeys(baseName, baseKey, location string) []string {
	var keys []string
	add := func(key string) {
		if slices.Contains(keys, key) {
			return
		}
		keys = append(keys, key)
	}
	baseURL, baseURLErr := url.Parse(baseName)
	baseIsURL := baseURLErr == nil && baseURL.Scheme != "" && baseURL.Opaque == "" && (baseURL.Host != "" || baseURL.Path != "")
	if baseIsURL {
		ref, err := url.Parse(location)
		if err == nil && ref.Opaque == "" && (ref.Scheme == "" || ref.Host != "" || ref.Path != "") {
			add(Key(baseURL.ResolveReference(ref).String()))
		}
	}
	if !baseIsURL {
		if resolved, ok := ResolveLocalSchemaLocation(baseKey, location); ok {
			add(Key(resolved))
		}
	}
	add(Key(location))
	return keys
}

// NormalizeSchemaLocation collapses XML whitespace in schemaLocation.
func NormalizeSchemaLocation(location string) (string, bool) {
	var b strings.Builder
	inWhitespace := true
	wrote := false
	for i := range len(location) {
		if lex.IsXMLWhitespaceByte(location[i]) {
			if wrote {
				inWhitespace = true
			}
			continue
		}
		if inWhitespace && wrote {
			b.WriteByte(' ')
		}
		b.WriteByte(location[i])
		wrote = true
		inWhitespace = false
	}
	if !wrote {
		return "", false
	}
	return b.String(), true
}

// ResolveLocalSchemaLocation resolves a local file schemaLocation against base.
func ResolveLocalSchemaLocation(base, location string) (string, bool) {
	if trimmed := lex.TrimXMLWhitespaceString(location); filepath.VolumeName(trimmed) != "" {
		return filepath.Clean(trimmed), true
	}
	u, err := url.Parse(location)
	if err == nil && u.Scheme != "" {
		return LocalFileURIPath(u)
	}
	location = filepath.FromSlash(lex.TrimXMLWhitespaceString(location))
	if location == "" {
		return "", false
	}
	return filepath.Clean(filepath.Join(filepath.Dir(base), location)), true
}

// LocalFileURIPath returns the local filesystem path represented by u.
func LocalFileURIPath(u *url.URL) (string, bool) {
	if u.Scheme != "file" {
		return "", false
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", false
	}
	file, err := url.PathUnescape(u.Path)
	if err != nil || file == "" {
		return "", false
	}
	return filepath.Clean(file), true
}
