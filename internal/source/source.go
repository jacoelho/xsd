// Package source defines internal schema source and resolver primitives.
package source

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/xsderrors"
)

// Source identifies a schema document passed to compilation.
type Source struct {
	resolver          *resolverOwner
	open              func(context.Context) (io.ReadCloser, error)
	name              string
	data              []byte
	localFileFallback bool
}

// ReferenceBase keeps the spelling presented to a custom resolver separate
// from the base that the built-in identity and file backends can represent.
// Applying xml:base may preserve a valid resolver base while making built-in
// fallback unavailable.
type ReferenceBase struct {
	resolver           string
	fallback           string
	resolverLocalPath  string
	resolverLocalQuery string
	resolverOK         bool
	fallbackOK         bool
	resolverLocal      bool
	resolverHasQuery   bool
}

// NewReferenceBase returns the unresolved base for one source context. The
// source name is preserved exactly for custom resolver callbacks until an
// xml:base value is applied.
func NewReferenceBase(name string) ReferenceBase {
	local := name != "" && isLocalName(name)
	return ReferenceBase{
		resolver:          name,
		fallback:          name,
		resolverLocalPath: name,
		resolverOK:        name != "",
		fallbackOK:        name != "",
		resolverLocal:     local,
	}
}

// ResolverValue returns the effective base to present to a custom resolver.
func (b ReferenceBase) ResolverValue() (string, bool) {
	return b.resolver, b.resolverOK
}

// WithXMLBase applies one xml:base value. Syntactically valid URI forms that
// cannot be represented by the built-in local backend remain available to a
// custom resolver without becoming document identities themselves.
func (b ReferenceBase) WithXMLBase(reference uriref.Reference) (ReferenceBase, error) {
	reference = reference.WithoutFragment()
	next := b
	if reference.Raw() == "" {
		if next.resolverOK && !next.resolverLocal {
			next.resolver = withoutFragment(next.resolver)
			next.resolverOK = next.resolver != ""
		}
		if next.fallbackOK && !isLocalName(next.fallback) {
			next.fallback = withoutFragment(next.fallback)
			next.fallbackOK = next.fallback != ""
		}
		return next, nil
	}
	resolver, err := resolveResolverBase(b, reference)
	if err != nil {
		return ReferenceBase{}, err
	}
	if !resolver.local {
		resolver.value = withoutFragment(resolver.value)
		resolver.ok = resolver.value != ""
	}
	next.resolver = resolver.value
	next.resolverLocalPath = resolver.localPath
	next.resolverLocalQuery = resolver.query
	next.resolverOK = resolver.ok
	next.resolverLocal = resolver.local
	next.resolverHasQuery = resolver.hasQuery

	fallback, fallbackOK, fallbackErr := resolveFallbackBase(b, reference)
	if fallbackErr != nil {
		return ReferenceBase{}, fallbackErr
	}
	if fallbackOK {
		if !isLocalName(fallback) {
			fallback = withoutFragment(fallback)
		}
		next.fallback = fallback
		next.fallbackOK = fallback != ""
	} else {
		next.fallback = ""
		next.fallbackOK = false
	}
	return next, nil
}

func resolveFallbackBase(base ReferenceBase, reference uriref.Reference) (string, bool, error) {
	fallbackBase := base.fallback
	fallbackOK := base.fallbackOK
	parts := reference.Parts()
	if !fallbackOK && parts.HasScheme {
		return canonicalFallbackReference(reference)
	}
	if !fallbackOK && base.resolverLocal && parts.Path != "" {
		fallbackBase = base.resolverLocalPath
		fallbackOK = fallbackBase != ""
	}
	if !fallbackOK {
		return "", false, nil
	}
	if isLocalName(fallbackBase) {
		resolved, err := ResolveReference(fallbackBase, reference.Escaped())
		if IsReferenceUnavailable(err) {
			return "", false, nil
		}
		return resolved, err == nil && resolved != "", err
	}
	if parts.HasScheme {
		return canonicalFallbackReference(reference)
	}
	baseReference, err := uriref.Parse(fallbackBase)
	if err != nil {
		return "", false, nil //nolint:nilerr // Arbitrary source names are identities, not schema-provided URI syntax.
	}
	resolved, err := uriref.Resolve(baseReference, reference)
	if errors.Is(err, uriref.ErrOpaqueBase) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return canonicalFallbackReference(resolved)
}

func canonicalFallbackReference(reference uriref.Reference) (string, bool, error) {
	resolved, err := ResolveReference("", reference.Escaped())
	if IsReferenceUnavailable(err) {
		return "", false, nil
	}
	return resolved, err == nil && resolved != "", err
}

func withoutFragment(uri string) string {
	if withoutFragment, _, present := strings.Cut(uri, "#"); present {
		return withoutFragment
	}
	return uri
}

type resolverOwner struct {
	resolve Resolver
}

var fileResolverOwner = &resolverOwner{}

func (o *resolverOwner) resolveSchema(ctx context.Context, base, location string) (Source, error) {
	return o.resolve.ResolveSchema(ctx, base, location)
}

// Resolver resolves schema include/import locations during compilation.
type Resolver func(ctx context.Context, base, location string) (Source, error)

// ResolveSchema resolves one schema include/import location.
func (r Resolver) ResolveSchema(ctx context.Context, base, location string) (Source, error) {
	if r == nil {
		return Source{}, xsderrors.ErrSchemaNotFound
	}
	return r(ctx, base, location)
}

// File returns a file schema source and resolves local schemaLocation refs.
func File(file string) Source {
	file = filepath.Clean(file)
	absolute, absoluteErr := filepath.Abs(file)
	if absoluteErr == nil {
		file = absolute
	}
	return Source{
		name: file,
		open: func(ctx context.Context) (io.ReadCloser, error) {
			if err := contextCause(ctx); err != nil {
				return nil, err
			}
			if absoluteErr != nil {
				return nil, absoluteErr
			}
			reader, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			return reader, nil
		},
		resolver:          fileResolverOwner,
		localFileFallback: true,
	}
}

// Bytes returns an in-memory schema source.
func Bytes(name string, data []byte) Source {
	if data == nil {
		data = []byte{}
	}
	return Source{name: name, data: bytes.Clone(data)}
}

// Opener returns a schema source backed by an opener.
func Opener(name string, open func(context.Context) (io.ReadCloser, error)) Source {
	return Source{name: name, open: open}
}

// WithResolver returns s with r used for schema include/import resolution.
func (s Source) WithResolver(r Resolver) Source {
	if r == nil {
		s.resolver = nil
	} else {
		s.resolver = &resolverOwner{resolve: r}
	}
	return s
}

// Name returns the source name.
func (s Source) Name() string {
	return s.name
}

// SameResolutionContext reports whether s and other resolve descendants with
// the same resolver owner and built-in backend capabilities.
func (s Source) SameResolutionContext(other Source) bool {
	return s.resolver == other.resolver && s.localFileFallback == other.localFileFallback
}

// Resolution is the result of resolving one schema reference. It contains a
// source when a backend supplied the referenced document, only a target when a
// generic document identity is representable, and neither when the valid
// reference is unavailable to the configured backends.
type Resolution struct {
	target string
	source Source
}

// Source returns the resolved source and whether a backend supplied it.
func (r Resolution) Source() (Source, bool) {
	return r.source, r.source.name != ""
}

// Target returns the singular referenced document identity, when one is
// representable independently of a backend.
func (r Resolution) Target() string {
	return r.target
}

// Resolve resolves location through s's attached resolver before applying
// generic URI-reference identity resolution. A resolver-returned source name
// is authoritative for the referenced document identity. The parent graph
// resolver owns resolution of references from returned sources.
func (s Source) Resolve(ctx context.Context, base, location string) (Resolution, error) {
	if err := contextCause(ctx); err != nil {
		return Resolution{}, err
	}
	reference, err := uriref.Parse(location)
	if err != nil {
		return Resolution{}, referenceResolutionError{err: err}
	}
	return s.ResolveFrom(ctx, NewReferenceBase(base), reference)
}

// ResolveFrom resolves location from a base whose custom-resolver spelling and
// built-in fallback capability have been tracked independently.
func (s Source) ResolveFrom(ctx context.Context, base ReferenceBase, location uriref.Reference) (Resolution, error) {
	if err := contextCause(ctx); err != nil {
		return Resolution{}, err
	}
	if s.resolver != nil && s.resolver != fileResolverOwner {
		if resolverBase, ok := base.ResolverValue(); ok {
			resolved, resolveErr := s.resolver.resolveSchema(ctx, resolverBase, location.Raw())
			if cause := contextCause(ctx); cause != nil {
				if resolveErr != nil {
					cause = errors.Join(cause, resolveErr)
				}
				return Resolution{}, cause
			}
			switch {
			case resolveErr == nil:
				if resolved.name == "" {
					return Resolution{}, errors.New("schema resolver returned a source without a name")
				}
				resolved.resolver = s.resolver
				return Resolution{source: resolved, target: Key(resolved.name)}, nil
			case !errorIsOnly(resolveErr, xsderrors.ErrSchemaNotFound):
				return Resolution{}, resolveErr
			}
		}
	}
	if location.HasFragment() {
		return Resolution{}, nil
	}
	resolvedBase, err := base.WithXMLBase(location)
	if err != nil {
		return Resolution{}, referenceResolutionError{err: err}
	}
	if !resolvedBase.fallbackOK {
		return Resolution{}, nil
	}
	target := resolvedBase.fallback
	if s.localFileFallback {
		if file, ok := localSchemaFile(target); ok {
			resolved := File(file)
			resolved.resolver = s.resolver
			return Resolution{source: resolved, target: Key(resolved.name)}, nil
		}
	}
	if target == "" {
		return Resolution{}, nil
	}
	return Resolution{target: Key(target)}, nil
}

type referenceResolutionError struct {
	err error
}

func (e referenceResolutionError) Error() string { return e.err.Error() }

func (e referenceResolutionError) Unwrap() error { return e.err }

// IsReferenceResolutionError reports whether err came from generic URI
// reference identity resolution rather than an attached schema resolver.
func IsReferenceResolutionError(err error) bool {
	var target referenceResolutionError
	return errors.As(err, &target)
}

// Read returns a copy of the source bytes.
func (s Source) Read(ctx context.Context, maxBytes int64) ([]byte, error) {
	result := s.Acquire(ctx, maxBytes)
	return bytes.Clone(result.Data), result.Err
}

// ReadStage identifies the source acquisition stage that failed.
type ReadStage uint8

const (
	// ReadStageOpen identifies an opener failure.
	ReadStageOpen ReadStage = iota + 1
	// ReadStageRead identifies a stream read failure.
	ReadStageRead
	// ReadStageClose identifies a stream close failure.
	ReadStageClose
)

// ReadResult reports a bounded source acquisition. Data aliases immutable
// Source storage for byte-backed sources and is loader-owned for opener-backed
// sources.
type ReadResult struct {
	Err           error
	Data          []byte
	Stage         ReadStage
	LimitExceeded bool
	// OpenNotFound reports an exclusively not-found opener error after successful cleanup.
	OpenNotFound bool
}

// Acquire reads at most maxBytes from s and preserves the failure stage and
// bytes consumed before an error.
func (s Source) Acquire(ctx context.Context, maxBytes int64) ReadResult {
	if err := contextCause(ctx); err != nil {
		return ReadResult{Err: err, Stage: ReadStageOpen}
	}
	if s.data != nil {
		if int64(len(s.data)) > maxBytes {
			return ReadResult{Err: schemaSourceLimitError(s.name), LimitExceeded: true}
		}
		return ReadResult{Data: s.data}
	}
	if s.open == nil {
		return ReadResult{
			Err:   xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source has no data or opener"),
			Stage: ReadStageOpen,
		}
	}
	r, err := s.open(ctx)
	if cause := contextCause(ctx); cause != nil {
		if err != nil {
			cause = errors.Join(cause, err)
		}
		if !isNilReadCloser(r) {
			if closeErr := r.Close(); closeErr != nil {
				cause = errors.Join(cause, closeErr)
			}
		}
		return ReadResult{Err: cause, Stage: ReadStageOpen}
	}
	if err != nil {
		openNotFound := errorIsOnly(err, os.ErrNotExist)
		if !isNilReadCloser(r) {
			if closeErr := r.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
				openNotFound = false
			}
		}
		return ReadResult{Err: err, Stage: ReadStageOpen, OpenNotFound: openNotFound}
	}
	if isNilReadCloser(r) {
		return ReadResult{
			Err:   xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema opener returned a nil reader"),
			Stage: ReadStageOpen,
		}
	}
	data, limitExceeded, readErr := readLimitedSchemaSource(ctx, s.name, r, maxBytes)
	closeErr := r.Close()
	if cause := contextCause(ctx); cause != nil {
		if readErr != nil {
			if errors.Is(readErr, cause) {
				cause = readErr
			} else {
				cause = errors.Join(cause, readErr)
			}
		}
		if closeErr != nil {
			cause = errors.Join(cause, closeErr)
		}
		return ReadResult{Data: data, LimitExceeded: limitExceeded, Err: cause, Stage: ReadStageRead}
	}
	if readErr != nil {
		if closeErr != nil {
			readErr = errors.Join(readErr, closeErr)
		}
		return ReadResult{Data: data, LimitExceeded: limitExceeded, Err: readErr, Stage: ReadStageRead}
	}
	if closeErr != nil {
		return ReadResult{Data: data, Err: closeErr, Stage: ReadStageClose}
	}
	return ReadResult{Data: data}
}

func isNilReadCloser(r io.ReadCloser) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func readLimitedSchemaSource(ctx context.Context, name string, r io.Reader, maxBytes int64) ([]byte, bool, error) {
	if maxBytes < 0 {
		return nil, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema reader byte limit cannot be negative")
	}
	reader := r
	if maxBytes < math.MaxInt64 {
		reader = io.LimitReader(r, maxBytes+1)
	}
	data, err := io.ReadAll(&schemaProgressReader{ctx: ctx, reader: reader})
	if int64(len(data)) > maxBytes {
		limitErr := schemaSourceLimitError(name)
		if err != nil {
			limitErr = errors.Join(limitErr, err)
		}
		return data, true, limitErr
	}
	if err != nil {
		return data, false, err
	}
	return data, false, nil
}

const maxConsecutiveEmptySchemaReads = 100

type schemaProgressReader struct {
	ctx        context.Context
	reader     io.Reader
	emptyReads int
}

func (r *schemaProgressReader) Read(p []byte) (int, error) {
	if err := contextCause(r.ctx); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if cause := contextCause(r.ctx); cause != nil {
		if err != nil {
			cause = errors.Join(cause, err)
		}
		return 0, cause
	}
	if n != 0 || err != nil {
		r.emptyReads = 0
		return n, err
	}
	r.emptyReads++
	if r.emptyReads >= maxConsecutiveEmptySchemaReads {
		return 0, io.ErrNoProgress
	}
	return 0, nil
}

func contextCause(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	return context.Cause(ctx)
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

// Key canonicalizes a schema source name for loaded-document identity.
func Key(name string) string {
	if isLocalName(name) {
		return canonicalLocalPath(name)
	}
	fragmentPresent := strings.IndexByte(name, '#') >= 0
	u, err := url.Parse(name)
	if err != nil {
		return name
	}
	authorityPresent := uriHasAuthoritySyntax(name, u.Scheme)
	if file, ok := localFileURIPath(u, fragmentPresent); ok {
		return file
	}
	if canonical, ok := canonicalURL(u, fragmentPresent, authorityPresent); ok {
		return canonical
	}
	return name
}

var errReferenceUnavailable = errors.New("schema reference is unavailable to the local backend")

// IsReferenceUnavailable reports whether a URI reference is syntactically
// valid but cannot be represented by the local source backend.
func IsReferenceUnavailable(err error) bool {
	return errors.Is(err, errReferenceUnavailable)
}

type resolverBaseState struct {
	value     string
	localPath string
	query     string
	ok        bool
	local     bool
	hasQuery  bool
}

func resolveResolverBase(base ReferenceBase, reference uriref.Reference) (resolverBaseState, error) {
	if base.resolverOK {
		if base.resolverLocal {
			return resolveLocalResolverBase(base, reference), nil
		}
		baseReference, err := uriref.Parse(base.resolver)
		if err != nil {
			if reference.Parts().HasScheme {
				return uriResolverBase(reference), nil
			}
			return resolverBaseState{}, nil
		}
		resolved, err := uriref.Resolve(baseReference, reference)
		if errors.Is(err, uriref.ErrOpaqueBase) {
			return resolverBaseState{}, nil
		}
		if err != nil {
			return resolverBaseState{}, err
		}
		return uriResolverBase(resolved), nil
	}
	if reference.Parts().HasScheme {
		return uriResolverBase(reference), nil
	}
	return resolverBaseState{}, nil
}

func uriResolverBase(reference uriref.Reference) resolverBaseState {
	return resolverBaseState{value: reference.Raw(), ok: reference.Raw() != ""}
}

func resolveLocalResolverBase(base ReferenceBase, reference uriref.Reference) resolverBaseState {
	parts := reference.Parts()
	if parts.HasScheme || parts.HasAuthority {
		return uriResolverBase(reference)
	}
	path := parts.Path
	if path == "" {
		query, hasQuery := base.resolverLocalQuery, base.resolverHasQuery
		if parts.HasQuery {
			query, hasQuery = parts.Query, true
		}
		return localResolverBase(base.resolverLocalPath, query, hasQuery)
	}
	if filepath.IsAbs(filepath.FromSlash(path)) || os.IsPathSeparator(path[0]) {
		path = filepath.FromSlash(path)
	} else {
		dir := filepath.Dir(base.resolverLocalPath)
		if localDirectoryForm(base.resolverLocalPath) {
			dir = base.resolverLocalPath
		}
		path = filepath.Join(dir, filepath.FromSlash(path))
	}
	path = canonicalLocalReference(path, localDirectoryForm(parts.Path))
	return localResolverBase(path, parts.Query, parts.HasQuery)
}

func localResolverBase(path, query string, hasQuery bool) resolverBaseState {
	value := path
	if hasQuery {
		value += "?" + query
	}
	return resolverBaseState{
		value: value, localPath: path, query: query, ok: value != "", local: true, hasQuery: hasQuery,
	}
}

// ResolveReference resolves one URI reference against base and returns its
// singular document identity.
func ResolveReference(base, reference string) (string, error) {
	baseLocal := isLocalName(base)
	if reference == "" {
		if baseLocal {
			return canonicalLocalReference(base, localDirectoryForm(base)), nil
		}
		baseURL, err := url.Parse(base)
		if err != nil {
			return "", err
		}
		fragmentPresent := strings.IndexByte(base, '#') >= 0
		authorityPresent, _, _ := uriAuthoritySyntax(base, baseURL.Scheme)
		if file, ok := localFileURIPath(baseURL, fragmentPresent); ok {
			return file, nil
		}
		canonical, ok := canonicalURL(baseURL, fragmentPresent, authorityPresent)
		if !ok {
			return "", errors.New("schema base has an invalid URI path")
		}
		return canonical, nil
	}
	if strings.IndexByte(reference, '#') >= 0 {
		return "", errors.New("schema reference fragments are not supported")
	}
	if baseLocal && filepath.VolumeName(reference) != "" && filepath.IsAbs(reference) {
		return canonicalLocalReference(reference, localDirectoryForm(reference)), nil
	}
	ref, err := url.Parse(reference)
	if err != nil {
		return "", err
	}
	refAuthority, emptyRefAuthority, emptyAuthorityPath := uriAuthoritySyntax(reference, ref.Scheme)
	if ref.Scheme != "" {
		if strings.EqualFold(ref.Scheme, "file") {
			ref.Scheme = "file"
			if !hasEncodedPathSeparator(ref.EscapedPath()) {
				if file, ok := localFileURIPath(ref, false); ok {
					return canonicalLocalReference(file, localDirectoryForm(ref.Path)), nil
				}
			}
		}
		canonical, ok := canonicalURL(ref, false, refAuthority)
		if !ok {
			return "", errors.New("schema reference has an invalid URI path")
		}
		return canonical, nil
	}
	if baseLocal {
		if refAuthority && (!emptyRefAuthority || emptyAuthorityPath == "") {
			return "", errReferenceUnavailable
		}
		if hasEncodedPathSeparator(ref.EscapedPath()) {
			return "", errReferenceUnavailable
		}
		if ref.Host != "" || ref.RawQuery != "" || ref.ForceQuery {
			return "", errReferenceUnavailable
		}
		refPath, decodeErr := url.PathUnescape(ref.EscapedPath())
		if decodeErr != nil {
			return "", errors.New("local schema reference has an invalid escaped path")
		}
		if strings.IndexByte(refPath, 0) >= 0 {
			return "", errReferenceUnavailable
		}
		basePath := base
		resolved := filepath.FromSlash(refPath)
		switch {
		case filepath.IsAbs(resolved):
		case resolved != "" && os.IsPathSeparator(resolved[0]):
			resolved = filepath.Join(filepath.VolumeName(basePath), resolved)
		default:
			resolved = filepath.Join(filepath.Dir(basePath), resolved)
		}
		return canonicalLocalReference(resolved, localDirectoryForm(refPath)), nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	baseAuthority := uriHasAuthoritySyntax(base, baseURL.Scheme)
	if refAuthority {
		return resolveAuthorityReference(baseURL.Scheme, ref, emptyRefAuthority, emptyAuthorityPath)
	}
	if baseURL.Opaque != "" {
		return "", errReferenceUnavailable
	}
	if ref.Opaque != "" {
		return "", errReferenceUnavailable
	}
	resolved := baseURL.ResolveReference(ref)
	canonical, ok := canonicalURL(resolved, false, baseAuthority)
	if !ok {
		return "", errors.New("schema reference has an invalid URI path")
	}
	return canonical, nil
}

func canonicalURL(parsed *url.URL, fragmentPresent, authorityPresent bool) (string, bool) {
	u := *parsed
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = canonicalURIHost(u.Host)
	if u.Opaque != "" {
		var ok bool
		u.Opaque, ok = canonicalEscapedComponent(u.Opaque)
		if !ok {
			return "", false
		}
	} else {
		escaped, ok := canonicalEscapedComponent(u.EscapedPath())
		if !ok {
			return "", false
		}
		escaped = removeURLDotSegments(escaped)
		decoded, err := url.PathUnescape(escaped)
		if err != nil {
			return "", false
		}
		u.Path = decoded
		u.RawPath = escaped
		plain := &url.URL{Path: decoded}
		if plain.EscapedPath() == escaped {
			u.RawPath = ""
		}
	}
	var ok bool
	u.RawQuery, ok = canonicalEscapedComponent(u.RawQuery)
	if !ok {
		return "", false
	}
	escapedFragment, ok := canonicalEscapedComponent(u.EscapedFragment())
	if !ok {
		return "", false
	}
	fragment, err := url.PathUnescape(escapedFragment)
	if err != nil {
		return "", false
	}
	u.Fragment = fragment
	u.RawFragment = escapedFragment
	plainFragment := &url.URL{Fragment: fragment}
	if plainFragment.EscapedFragment() == escapedFragment {
		u.RawFragment = ""
	}
	canonical := u.String()
	if authorityPresent && u.Opaque == "" && u.Host == "" && u.User == nil {
		start := 0
		if u.Scheme != "" {
			start = len(u.Scheme) + 1
		}
		if !strings.HasPrefix(canonical[start:], "//") {
			canonical = canonical[:start] + "//" + canonical[start:]
		}
	}
	if fragmentPresent && u.Fragment == "" {
		canonical += "#"
	}
	return canonical, true
}

func canonicalURIHost(host string) string {
	if !strings.HasPrefix(host, "[") {
		return strings.ToLower(host)
	}
	closingBracket := strings.LastIndexByte(host, ']')
	if closingBracket < 0 {
		return strings.ToLower(host)
	}
	literal := host[1:closingBracket]
	zone := strings.IndexByte(literal, '%')
	if zone < 0 {
		return strings.ToLower(host)
	}
	return "[" + strings.ToLower(literal[:zone]) + literal[zone:] + host[closingBracket:]
}

func uriAuthoritySyntax(raw, scheme string) (present, empty bool, escapedPath string) {
	rest := raw
	if scheme != "" {
		_, after, ok := strings.Cut(raw, ":")
		if !ok {
			return false, false, ""
		}
		rest = after
	}
	if !strings.HasPrefix(rest, "//") {
		return false, false, ""
	}
	hierarchy := rest[2:]
	if end := strings.IndexAny(hierarchy, "?#"); end >= 0 {
		hierarchy = hierarchy[:end]
	}
	if hierarchy == "" {
		return true, true, ""
	}
	if hierarchy[0] == '/' {
		return true, true, hierarchy
	}
	return true, false, ""
}

func uriHasAuthoritySyntax(raw, scheme string) bool {
	present, _, _ := uriAuthoritySyntax(raw, scheme) //nolint:dogsled // Only delimiter presence is needed here.
	return present
}

func resolveAuthorityReference(scheme string, ref *url.URL, empty bool, escapedPath string) (string, error) {
	if empty {
		decodedPath, err := url.PathUnescape(escapedPath)
		if err != nil || strings.IndexByte(decodedPath, 0) >= 0 {
			return "", errors.New("schema reference has an invalid escaped path")
		}
		ref.Path = decodedPath
		ref.RawPath = escapedPath
		plain := &url.URL{Path: decodedPath}
		if plain.EscapedPath() == escapedPath {
			ref.RawPath = ""
		}
	}
	ref.Scheme = scheme
	canonical, ok := canonicalURL(ref, false, true)
	if !ok {
		return "", errors.New("schema reference has an invalid URI path")
	}
	return canonical, nil
}

// removeURLDotSegments applies RFC 3986 path resolution without collapsing
// empty segments, which remain identity-significant for hierarchical URIs.
func removeURLDotSegments(escaped string) string {
	if escaped == "" {
		return ""
	}
	leadingSlash := escaped[0] == '/'
	parts := strings.Split(escaped, "/")
	stack := make([]string, 0, len(parts))
	for _, elem := range parts {
		switch elem {
		case ".":
		case "..":
			if len(stack) != 0 && (len(stack) != 1 || stack[0] != "") {
				stack = stack[:len(stack)-1]
			}
		default:
			stack = append(stack, elem)
		}
	}
	last := parts[len(parts)-1]
	if last == "." || last == ".." {
		stack = append(stack, "")
	}
	cleaned := strings.Join(stack, "/")
	if leadingSlash && !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
		if cleaned == "/" && len(stack) > 1 {
			cleaned += strings.Repeat("/", len(stack)-1)
		}
	}
	return cleaned
}

func canonicalEscapedComponent(escaped string) (string, bool) {
	if !strings.Contains(escaped, "%") {
		return escaped, true
	}
	var b strings.Builder
	b.Grow(len(escaped))
	for i := 0; i < len(escaped); i++ {
		if escaped[i] != '%' {
			b.WriteByte(escaped[i])
			continue
		}
		if i+2 >= len(escaped) {
			return "", false
		}
		hi, ok := hexValue(escaped[i+1])
		if !ok {
			return "", false
		}
		lo, ok := hexValue(escaped[i+2])
		if !ok {
			return "", false
		}
		value := hi<<4 | lo
		if isURIUnreserved(value) {
			b.WriteByte(value)
		} else {
			const upperHex = "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(upperHex[value>>4])
			b.WriteByte(upperHex[value&0xf])
		}
		i += 2
	}
	return b.String(), true
}

func hexValue(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

func isURIUnreserved(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' ||
		b >= '0' && b <= '9' || b == '-' || b == '.' || b == '_' || b == '~'
}

func hasEncodedPathSeparator(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "%2f") || strings.Contains(lower, "%00") ||
		os.IsPathSeparator('\\') && strings.Contains(lower, "%5c")
}

func canonicalLocalPath(name string) string {
	cleaned := filepath.Clean(name)
	if filepath.IsAbs(cleaned) {
		return cleaned
	}
	if hasURIScheme(filepath.ToSlash(cleaned)) {
		return "." + string(filepath.Separator) + cleaned
	}
	return cleaned
}

func canonicalLocalReference(name string, directory bool) string {
	cleaned := canonicalLocalPath(name)
	if directory && !os.IsPathSeparator(cleaned[len(cleaned)-1]) {
		cleaned += string(filepath.Separator)
	}
	return cleaned
}

func localDirectoryForm(name string) bool {
	if name == "" {
		return false
	}
	if os.IsPathSeparator(name[len(name)-1]) {
		return true
	}
	start := len(name)
	for start > 0 && !os.IsPathSeparator(name[start-1]) {
		start--
	}
	last := name[start:]
	return last == "." || last == ".."
}

func isLocalName(name string) bool {
	return filepath.IsAbs(name) || !hasURIScheme(name)
}

func hasURIScheme(name string) bool {
	if len(name) < 2 || !isASCIIAlpha(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		b := name[i]
		if b == ':' {
			return true
		}
		if !isASCIIAlpha(b) && (b < '0' || b > '9') && b != '+' && b != '-' && b != '.' {
			return false
		}
	}
	return false
}

func isASCIIAlpha(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func errorIsOnly(err, target error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !errorIsOnly(cause, target) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		if cause := wrapped.Unwrap(); cause != nil {
			return errorIsOnly(cause, target)
		}
	}
	return errors.Is(err, target)
}

func localSchemaFile(resolved string) (string, bool) {
	u, err := url.Parse(resolved)
	if err == nil && u.Scheme != "" {
		return localFileURIPath(u, strings.IndexByte(resolved, '#') >= 0)
	}
	if !isLocalName(resolved) {
		return "", false
	}
	return canonicalLocalPath(resolved), true
}

// localFileURIPath returns the local filesystem path represented by u.
// fragmentPresent carries syntax that net/url does not retain for a trailing '#'.
func localFileURIPath(u *url.URL, fragmentPresent bool) (string, bool) {
	if !strings.EqualFold(u.Scheme, "file") || u.User != nil || u.RawQuery != "" || u.ForceQuery || fragmentPresent || u.Fragment != "" {
		return "", false
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", false
	}
	if hasEncodedPathSeparator(u.EscapedPath()) || u.Path == "" || strings.IndexByte(u.Path, 0) >= 0 {
		return "", false
	}
	file := u.Path
	if filepath.Separator == '\\' && len(file) >= 3 && file[0] == '/' && file[2] == ':' {
		file = file[1:]
	}
	return filepath.Clean(filepath.FromSlash(file)), true
}
