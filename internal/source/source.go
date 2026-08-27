// Package source defines internal schema source and resolver primitives.
package source

import (
	"bytes"
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
	open    func() (io.ReadCloser, error)
	context resolutionContext
	name    string
	data    []byte
	kind    sourceKind
}

type sourceKind uint8

const (
	sourceInvalid sourceKind = iota
	sourceBytes
	sourceOpener
)

type resolutionContext struct {
	resolver          *resolverOwner
	localFileFallback bool
}

// ReferenceBase keeps the spelling presented to a custom resolver separate
// from the base that the built-in identity and file backends can represent.
// Applying xml:base may preserve a valid resolver base while making built-in
// fallback unavailable.
type ReferenceBase struct {
	fallback string
	resolver resolverBase
}

type resolverBaseKind uint8

const (
	resolverBaseUnavailable resolverBaseKind = iota
	resolverBaseURI
	resolverBaseLocal
)

type resolverBase struct {
	value     string
	localPath string
	query     string
	kind      resolverBaseKind
	hasQuery  bool
}

func (b resolverBase) available() bool {
	return b.kind != resolverBaseUnavailable
}

// NewReferenceBase returns the unresolved base for one source context. The
// source name is preserved exactly for custom resolver callbacks until an
// xml:base value is applied.
func NewReferenceBase(name string) ReferenceBase {
	return ReferenceBase{
		resolver: newResolverBase(name),
		fallback: name,
	}
}

func newResolverBase(value string) resolverBase {
	if value == "" {
		return resolverBase{}
	}
	if isLocalName(value) {
		return localResolverBase(value, "", false)
	}
	return resolverBase{value: value, kind: resolverBaseURI}
}

// ResolverValue returns the effective base to present to a custom resolver.
func (b ReferenceBase) ResolverValue() (string, bool) {
	return b.resolver.value, b.resolver.available()
}

// WithXMLBase applies one xml:base value. Syntactically valid URI forms that
// cannot be represented by the built-in local backend remain available to a
// custom resolver without becoming document identities themselves.
func (b ReferenceBase) WithXMLBase(reference uriref.Reference) (ReferenceBase, error) {
	reference = reference.WithoutFragment()
	if reference.Raw() == "" {
		return b.withoutFragment(), nil
	}
	resolver, err := resolveResolverBase(b, reference)
	if err != nil {
		return ReferenceBase{}, err
	}
	fallback, fallbackErr := resolveFallbackBase(b, reference)
	if fallbackErr != nil {
		return ReferenceBase{}, fallbackErr
	}
	return (ReferenceBase{resolver: resolver, fallback: fallback}).withoutFragment(), nil
}

func (b ReferenceBase) withoutFragment() ReferenceBase {
	if b.resolver.kind == resolverBaseURI {
		b.resolver = uriResolverBaseValue(withoutFragment(b.resolver.value))
	}
	if b.fallback != "" && !isLocalName(b.fallback) {
		b.fallback = withoutFragment(b.fallback)
	}
	return b
}

func resolveFallbackBase(base ReferenceBase, reference uriref.Reference) (string, error) {
	if base.fallback == "" {
		return resolveFallbackWithoutBase(base, reference)
	}
	return resolveFallbackAgainst(base.fallback, reference)
}

func resolveFallbackWithoutBase(base ReferenceBase, reference uriref.Reference) (string, error) {
	parts := reference.Parts()
	if parts.HasScheme {
		return canonicalFallbackReference(reference)
	}
	if base.resolver.kind != resolverBaseLocal || parts.Path == "" {
		return "", nil
	}
	return resolveFallbackAgainst(base.resolver.localPath, reference)
}

func resolveFallbackAgainst(fallbackBase string, reference uriref.Reference) (string, error) {
	if isLocalName(fallbackBase) {
		resolved, err := ResolveReference(fallbackBase, reference.Escaped())
		if IsReferenceUnavailable(err) {
			return "", nil
		}
		return resolved, err
	}
	if reference.Parts().HasScheme {
		return canonicalFallbackReference(reference)
	}
	baseReference, err := uriref.Parse(fallbackBase)
	if err != nil {
		return "", nil //nolint:nilerr // Arbitrary source names are identities, not schema-provided URI syntax.
	}
	resolved, err := uriref.Resolve(baseReference, reference)
	if errors.Is(err, uriref.ErrOpaqueBase) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return canonicalFallbackReference(resolved)
}

func canonicalFallbackReference(reference uriref.Reference) (string, error) {
	resolved, err := ResolveReference("", reference.Escaped())
	if IsReferenceUnavailable(err) {
		return "", nil
	}
	return resolved, err
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

func (o *resolverOwner) resolveSchema(base, location string) (Source, error) {
	return o.resolve.ResolveSchema(base, location)
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
	absolute, absoluteErr := filepath.Abs(file)
	if absoluteErr == nil {
		file = absolute
	}
	return Source{
		name: file,
		kind: sourceOpener,
		open: func() (io.ReadCloser, error) {
			if absoluteErr != nil {
				return nil, absoluteErr
			}
			reader, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			return reader, nil
		},
		context: resolutionContext{resolver: fileResolverOwner, localFileFallback: true},
	}
}

// Bytes returns an in-memory schema source.
func Bytes(name string, data []byte) Source {
	if data == nil {
		data = []byte{}
	}
	return Source{name: name, data: bytes.Clone(data), kind: sourceBytes}
}

// Opener returns a schema source backed by an opener.
func Opener(name string, open func() (io.ReadCloser, error)) Source {
	return Source{name: name, open: open, kind: sourceOpener}
}

// WithResolver returns s with r used for schema include/import resolution.
func (s Source) WithResolver(r Resolver) Source {
	if r == nil {
		s.context.resolver = nil
	} else {
		s.context.resolver = &resolverOwner{resolve: r}
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
	return s.context == other.context
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

// ResolveFrom resolves location from a base whose custom-resolver spelling and
// built-in fallback capability have been tracked independently.
func (s Source) ResolveFrom(base ReferenceBase, location uriref.Reference) (Resolution, error) {
	resolved, err := s.resolveWithCustomResolver(base, location)
	if err != nil {
		return Resolution{}, err
	}
	if _, ok := resolved.Source(); ok {
		return resolved, nil
	}
	return s.resolveWithBuiltins(base, location)
}

func (s Source) resolveWithCustomResolver(base ReferenceBase, location uriref.Reference) (Resolution, error) {
	if s.context.resolver == nil || s.context.resolver == fileResolverOwner {
		return Resolution{}, nil
	}
	resolverBase, ok := base.ResolverValue()
	if !ok {
		return Resolution{}, nil
	}
	resolved, err := s.context.resolver.resolveSchema(resolverBase, location.Raw())
	if err != nil {
		if errorIsOnly(err, xsderrors.ErrSchemaNotFound) {
			return Resolution{}, nil
		}
		return Resolution{}, err
	}
	if resolved.name == "" {
		return Resolution{}, errors.New("schema resolver returned a source without a name")
	}
	resolved.context.resolver = s.context.resolver
	return Resolution{source: resolved, target: Key(resolved.name)}, nil
}

func (s Source) resolveWithBuiltins(base ReferenceBase, location uriref.Reference) (Resolution, error) {
	if location.HasFragment() {
		return Resolution{}, nil
	}
	resolvedBase, err := base.WithXMLBase(location)
	if err != nil {
		return Resolution{}, referenceResolutionError{err: err}
	}
	if resolvedBase.fallback == "" {
		return Resolution{}, nil
	}
	target := resolvedBase.fallback
	if s.context.localFileFallback {
		if file, ok := localSchemaFile(target); ok {
			resolved := File(file)
			resolved.context.resolver = s.context.resolver
			return Resolution{source: resolved, target: Key(resolved.name)}, nil
		}
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
func (s Source) Acquire(maxBytes int64) ReadResult {
	switch s.kind {
	case sourceBytes:
		if int64(len(s.data)) > maxBytes {
			return ReadResult{Err: schemaSourceLimitError(s.name), LimitExceeded: true}
		}
		return ReadResult{Data: s.data}
	case sourceOpener:
		if s.open == nil {
			return ReadResult{
				Err:   xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source opener is nil"),
				Stage: ReadStageOpen,
			}
		}
		return s.acquireOpenedSource(maxBytes)
	default:
		return ReadResult{
			Err:   xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source is invalid"),
			Stage: ReadStageOpen,
		}
	}
}

func (s Source) acquireOpenedSource(maxBytes int64) ReadResult {
	r, err := s.open()
	if err != nil {
		return openSourceFailure(r, err)
	}
	if isNilReadCloser(r) {
		return ReadResult{
			Err:   xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema opener returned a nil reader"),
			Stage: ReadStageOpen,
		}
	}
	return readAndCloseSource(s.name, r, maxBytes)
}

func openSourceFailure(r io.ReadCloser, err error) ReadResult {
	openNotFound := errorIsOnly(err, os.ErrNotExist)
	if !isNilReadCloser(r) {
		if closeErr := r.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
			openNotFound = false
		}
	}
	return ReadResult{Err: err, Stage: ReadStageOpen, OpenNotFound: openNotFound}
}

func readAndCloseSource(name string, r io.ReadCloser, maxBytes int64) ReadResult {
	data, limitExceeded, readErr := readLimitedSchemaSource(name, r, maxBytes)
	closeErr := r.Close()
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

func readLimitedSchemaSource(name string, r io.Reader, maxBytes int64) ([]byte, bool, error) {
	if maxBytes < 0 {
		return nil, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema reader byte limit cannot be negative")
	}
	reader := r
	if maxBytes < math.MaxInt64 {
		reader = io.LimitReader(r, maxBytes+1)
	}
	data, err := io.ReadAll(&schemaProgressReader{reader: reader})
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
	reader     io.Reader
	emptyReads int
}

func (r *schemaProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
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
	return ok && x.Code() == xsderrors.CodeSchemaLimit
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

func resolveResolverBase(base ReferenceBase, reference uriref.Reference) (resolverBase, error) {
	if base.resolver.available() {
		return resolveAvailableResolverBase(base.resolver, reference)
	}
	if reference.Parts().HasScheme {
		return uriResolverBase(reference), nil
	}
	return resolverBase{}, nil
}

func resolveAvailableResolverBase(base resolverBase, reference uriref.Reference) (resolverBase, error) {
	if base.kind == resolverBaseLocal {
		return resolveLocalResolverBase(base, reference), nil
	}
	baseReference, err := uriref.Parse(base.value)
	if err != nil {
		if reference.Parts().HasScheme {
			return uriResolverBase(reference), nil
		}
		return resolverBase{}, nil
	}
	resolved, err := uriref.Resolve(baseReference, reference)
	if errors.Is(err, uriref.ErrOpaqueBase) {
		return resolverBase{}, nil
	}
	if err != nil {
		return resolverBase{}, err
	}
	return uriResolverBase(resolved), nil
}

func uriResolverBase(reference uriref.Reference) resolverBase {
	return uriResolverBaseValue(reference.Raw())
}

func uriResolverBaseValue(value string) resolverBase {
	if value == "" {
		return resolverBase{}
	}
	return resolverBase{value: value, kind: resolverBaseURI}
}

func resolveLocalResolverBase(base resolverBase, reference uriref.Reference) resolverBase {
	parts := reference.Parts()
	if parts.HasScheme || parts.HasAuthority {
		return uriResolverBase(reference)
	}
	path := parts.Path
	if path == "" {
		query, hasQuery := base.query, base.hasQuery
		if parts.HasQuery {
			query, hasQuery = parts.Query, true
		}
		return localResolverBase(base.localPath, query, hasQuery)
	}
	if filepath.IsAbs(filepath.FromSlash(path)) || os.IsPathSeparator(path[0]) {
		path = filepath.FromSlash(path)
	} else {
		dir := filepath.Dir(base.localPath)
		if localDirectoryForm(base.localPath) {
			dir = base.localPath
		}
		path = filepath.Join(dir, filepath.FromSlash(path))
	}
	path = canonicalLocalReference(path, localDirectoryForm(parts.Path))
	return localResolverBase(path, parts.Query, parts.HasQuery)
}

func localResolverBase(path, query string, hasQuery bool) resolverBase {
	value := path
	if hasQuery {
		value += "?" + query
	}
	if value == "" {
		return resolverBase{}
	}
	return resolverBase{
		value: value, localPath: path, query: query, kind: resolverBaseLocal, hasQuery: hasQuery,
	}
}

// ResolveReference resolves one URI reference against base and returns its
// singular document identity.
func ResolveReference(base, reference string) (string, error) {
	baseLocal := isLocalName(base)
	if reference == "" {
		return resolveEmptyReference(base, baseLocal)
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
		return resolveAbsoluteReference(ref, refAuthority)
	}
	if baseLocal {
		return resolveLocalReference(base, ref, refAuthority, emptyRefAuthority, emptyAuthorityPath)
	}
	return resolveURIReference(base, ref, refAuthority, emptyRefAuthority, emptyAuthorityPath)
}

func resolveEmptyReference(base string, local bool) (string, error) {
	if local {
		return canonicalLocalReference(base, localDirectoryForm(base)), nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	fragmentPresent := strings.IndexByte(base, '#') >= 0
	authorityPresent := uriHasAuthoritySyntax(base, baseURL.Scheme)
	if file, ok := localFileURIPath(baseURL, fragmentPresent); ok {
		return file, nil
	}
	canonical, ok := canonicalURL(baseURL, fragmentPresent, authorityPresent)
	if !ok {
		return "", errors.New("schema base has an invalid URI path")
	}
	return canonical, nil
}

func resolveAbsoluteReference(ref *url.URL, authorityPresent bool) (string, error) {
	if strings.EqualFold(ref.Scheme, "file") {
		ref.Scheme = "file"
		if !hasEncodedPathSeparator(ref.EscapedPath()) {
			if file, ok := localFileURIPath(ref, false); ok {
				return canonicalLocalReference(file, localDirectoryForm(ref.Path)), nil
			}
		}
	}
	canonical, ok := canonicalURL(ref, false, authorityPresent)
	if !ok {
		return "", errors.New("schema reference has an invalid URI path")
	}
	return canonical, nil
}

func resolveLocalReference(base string, ref *url.URL, authority, emptyAuthority bool, authorityPath string) (string, error) {
	if authority && (!emptyAuthority || authorityPath == "") {
		return "", errReferenceUnavailable
	}
	if hasEncodedPathSeparator(ref.EscapedPath()) {
		return "", errReferenceUnavailable
	}
	if ref.Host != "" || ref.RawQuery != "" || ref.ForceQuery {
		return "", errReferenceUnavailable
	}
	refPath, err := url.PathUnescape(ref.EscapedPath())
	if err != nil {
		return "", errors.New("local schema reference has an invalid escaped path")
	}
	if strings.IndexByte(refPath, 0) >= 0 {
		return "", errReferenceUnavailable
	}
	return resolveLocalReferencePath(base, refPath), nil
}

func resolveLocalReferencePath(base, refPath string) string {
	resolved := filepath.FromSlash(refPath)
	switch {
	case filepath.IsAbs(resolved):
	case resolved != "" && os.IsPathSeparator(resolved[0]):
		resolved = filepath.Join(filepath.VolumeName(base), resolved)
	default:
		resolved = filepath.Join(filepath.Dir(base), resolved)
	}
	return canonicalLocalReference(resolved, localDirectoryForm(refPath))
}

func resolveURIReference(base string, ref *url.URL, authority, emptyAuthority bool, authorityPath string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if authority {
		return resolveAuthorityReference(baseURL.Scheme, ref, emptyAuthority, authorityPath)
	}
	if baseURL.Opaque != "" || ref.Opaque != "" {
		return "", errReferenceUnavailable
	}
	resolved := baseURL.ResolveReference(ref)
	canonical, ok := canonicalURL(resolved, false, uriHasAuthoritySyntax(base, baseURL.Scheme))
	if !ok {
		return "", errors.New("schema reference has an invalid URI path")
	}
	return canonical, nil
}

func canonicalURL(parsed *url.URL, fragmentPresent, authorityPresent bool) (string, bool) {
	u := *parsed
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = canonicalURIHost(u.Host)
	if !canonicalizeURLPath(&u) {
		return "", false
	}
	query, ok := canonicalEscapedComponent(u.RawQuery)
	if !ok {
		return "", false
	}
	u.RawQuery = query
	if !canonicalizeURLFragment(&u) {
		return "", false
	}
	canonical := preserveAuthoritySyntax(u.String(), &u, authorityPresent)
	if fragmentPresent && u.Fragment == "" {
		canonical += "#"
	}
	return canonical, true
}

func canonicalizeURLPath(u *url.URL) bool {
	if u.Opaque != "" {
		opaque, ok := canonicalEscapedComponent(u.Opaque)
		if !ok {
			return false
		}
		u.Opaque = opaque
		return true
	}
	escaped, ok := canonicalEscapedComponent(u.EscapedPath())
	if !ok {
		return false
	}
	escaped = removeURLDotSegments(escaped)
	decoded, err := url.PathUnescape(escaped)
	if err != nil {
		return false
	}
	u.Path = decoded
	u.RawPath = escaped
	if (&url.URL{Path: decoded}).EscapedPath() == escaped {
		u.RawPath = ""
	}
	return true
}

func canonicalizeURLFragment(u *url.URL) bool {
	escapedFragment, ok := canonicalEscapedComponent(u.EscapedFragment())
	if !ok {
		return false
	}
	fragment, err := url.PathUnescape(escapedFragment)
	if err != nil {
		return false
	}
	u.Fragment = fragment
	u.RawFragment = escapedFragment
	if (&url.URL{Fragment: fragment}).EscapedFragment() == escapedFragment {
		u.RawFragment = ""
	}
	return true
}

func preserveAuthoritySyntax(canonical string, u *url.URL, authorityPresent bool) string {
	if !authorityPresent || u.Opaque != "" || u.Host != "" || u.User != nil {
		return canonical
	}
	start := 0
	if u.Scheme != "" {
		start = len(u.Scheme) + 1
	}
	if strings.HasPrefix(canonical[start:], "//") {
		return canonical
	}
	return canonical[:start] + "//" + canonical[start:]
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
		stack = applyURLDotSegment(stack, elem)
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

func applyURLDotSegment(stack []string, elem string) []string {
	switch elem {
	case ".":
		return stack
	case "..":
		if len(stack) != 0 && (len(stack) != 1 || stack[0] != "") {
			return stack[:len(stack)-1]
		}
		return stack
	default:
		return append(stack, elem)
	}
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
		ok := appendCanonicalEscape(&b, escaped[i:])
		if !ok {
			return "", false
		}
		i += 2
	}
	return b.String(), true
}

func appendCanonicalEscape(b *strings.Builder, escaped string) bool {
	if len(escaped) < 3 {
		return false
	}
	hi, hiOK := hexValue(escaped[1])
	lo, loOK := hexValue(escaped[2])
	if !hiOK || !loOK {
		return false
	}
	value := hi<<4 | lo
	if isURIUnreserved(value) {
		b.WriteByte(value)
		return true
	}
	const upperHex = "0123456789ABCDEF"
	b.WriteByte('%')
	b.WriteByte(upperHex[value>>4])
	b.WriteByte(upperHex[value&0xf])
	return true
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
		return errorsAreOnly(joined.Unwrap(), target)
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		if cause := wrapped.Unwrap(); cause != nil {
			return errorIsOnly(cause, target)
		}
	}
	return errors.Is(err, target)
}

func errorsAreOnly(causes []error, target error) bool {
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
