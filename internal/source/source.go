// Package source defines internal schema source and resolver primitives.
package source

import (
	"bytes"
	"cmp"
	"errors"
	"hash/maphash"
	"io"
	"maps"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// ErrNilReader reports a nil schema reader.
var ErrNilReader = errors.New("nil schema reader")

// SourceNameIssue identifies an invalid schema source name.
type SourceNameIssue uint8

const (
	// SourceNameOK reports a valid schema source name.
	SourceNameOK SourceNameIssue = iota
	// SourceNameMissing reports an unnamed schema source.
	SourceNameMissing
)

// Code returns the schema compile error code for issue.
func (i SourceNameIssue) Code() xsderrors.Code {
	switch i {
	case SourceNameMissing:
		return xsderrors.CodeSchemaRead
	default:
		return ""
	}
}

// Message returns the schema compile diagnostic text for issue.
func (i SourceNameIssue) Message() string {
	switch i {
	case SourceNameMissing:
		return "schema source name is required"
	default:
		return ""
	}
}

// CheckSourceName validates a schema source name before loading.
func CheckSourceName(name string) SourceNameIssue {
	if name == "" {
		return SourceNameMissing
	}
	return SourceNameOK
}

// ImportNamespaceIssue identifies an invalid explicit xs:import namespace.
type ImportNamespaceIssue uint8

const (
	// ImportNamespaceOK reports a valid explicit import namespace.
	ImportNamespaceOK ImportNamespaceIssue = iota
	// ImportNamespaceEmpty reports namespace="" on xs:import.
	ImportNamespaceEmpty
	// ImportNamespaceMissingTarget reports no namespace on a no-target schema.
	ImportNamespaceMissingTarget
	// ImportNamespaceMatchesTarget reports an import of the enclosing target.
	ImportNamespaceMatchesTarget
)

// Code returns the schema compile error code for issue.
func (i ImportNamespaceIssue) Code() xsderrors.Code {
	switch i {
	case ImportNamespaceEmpty:
		return xsderrors.CodeSchemaInvalidAttribute
	case ImportNamespaceMissingTarget, ImportNamespaceMatchesTarget:
		return xsderrors.CodeSchemaReference
	default:
		return ""
	}
}

// Message returns the schema compile diagnostic text for issue.
func (i ImportNamespaceIssue) Message() string {
	switch i {
	case ImportNamespaceEmpty:
		return "import namespace cannot be empty"
	case ImportNamespaceMissingTarget:
		return "import without namespace requires enclosing schema targetNamespace"
	case ImportNamespaceMatchesTarget:
		return "import namespace cannot match enclosing schema targetNamespace"
	default:
		return ""
	}
}

// CheckImportNamespace validates the namespace selected by an explicit
// xs:import declaration.
func CheckImportNamespace(target, namespace string, hasNamespace bool) (string, ImportNamespaceIssue) {
	if hasNamespace && namespace == "" {
		return "", ImportNamespaceEmpty
	}
	if !hasNamespace && target == "" {
		return "", ImportNamespaceMissingTarget
	}
	if hasNamespace && namespace == target {
		return "", ImportNamespaceMatchesTarget
	}
	return namespace, ImportNamespaceOK
}

// IncludeLocationIssue identifies an invalid explicit xs:include location.
type IncludeLocationIssue uint8

const (
	// IncludeLocationOK reports a valid explicit include location.
	IncludeLocationOK IncludeLocationIssue = iota
	// IncludeLocationMissing reports an xs:include without schemaLocation.
	IncludeLocationMissing
)

// Code returns the schema compile error code for issue.
func (i IncludeLocationIssue) Code() xsderrors.Code {
	switch i {
	case IncludeLocationMissing:
		return xsderrors.CodeSchemaReference
	default:
		return ""
	}
}

// Message returns the schema compile diagnostic text for issue.
func (i IncludeLocationIssue) Message() string {
	switch i {
	case IncludeLocationMissing:
		return "include missing schemaLocation"
	default:
		return ""
	}
}

// CheckIncludeSchemaLocation validates that an explicit xs:include has a
// usable schemaLocation after lexical normalization.
func CheckIncludeSchemaLocation(hasLocation bool) IncludeLocationIssue {
	if !hasLocation {
		return IncludeLocationMissing
	}
	return IncludeLocationOK
}

// SchemaLocationAttr reads and normalizes an XSD schemaLocation attribute.
func SchemaLocationAttr(attr func(string) (string, bool)) (string, bool) {
	if attr == nil {
		return "", false
	}
	location, ok := attr(vocab.XSDAttrSchemaLocation)
	if !ok {
		return "", false
	}
	return NormalizeSchemaLocation(location)
}

// SchemaReferenceElement is the source-relevant projection of one top-level
// schema child.
type SchemaReferenceElement struct {
	Attr   func(string) (string, bool)
	Local  string
	Line   int
	Column int
}

// SchemaDocumentReference is one normalized include/import schemaLocation
// reference from a schema document.
type SchemaDocumentReference struct {
	Location  string
	Namespace string
	Line      int
	Column    int
	Kind      SchemaReferenceKind
}

// SchemaReferenceKind identifies the source-level meaning of a schemaLocation
// reference.
type SchemaReferenceKind uint8

const (
	// SchemaReferenceInclude reports an xs:include schemaLocation.
	SchemaReferenceInclude SchemaReferenceKind = iota
	// SchemaReferenceImport reports an xs:import schemaLocation.
	SchemaReferenceImport
)

// SchemaDocumentReferences extracts include/import schemaLocation references
// from top-level schema children.
func SchemaDocumentReferences(elements []SchemaReferenceElement) []SchemaDocumentReference {
	var refs []SchemaDocumentReference
	for _, elem := range elements {
		switch elem.Local {
		case vocab.XSDElemInclude, vocab.XSDElemImport:
			location, ok := SchemaLocationAttr(elem.Attr)
			if !ok {
				continue
			}
			ref := SchemaDocumentReference{
				Kind:     SchemaReferenceInclude,
				Location: location,
				Line:     elem.Line,
				Column:   elem.Column,
			}
			if elem.Local == vocab.XSDElemImport {
				ref.Kind = SchemaReferenceImport
				ref.Namespace, _ = elem.Attr(vocab.XSDAttrNamespace)
			}
			refs = append(refs, ref)
		}
	}
	return refs
}

// LoadRequest is one schema source to read during transitive source loading.
type LoadRequest struct {
	Source          Source
	OptionalMissing bool
}

// LoadedSource is the raw source data passed to the schema document parser
// during source loading.
type LoadedSource struct {
	Name string
	Key  string
	Data []byte
}

// LoadedSchemaDocument is the source-relevant projection of a parsed schema
// document.
type LoadedSchemaDocument struct {
	TargetNamespace string
	References      []SchemaDocumentReference
}

// LoadSchemaDocumentFunc parses one loaded source and returns its source-level
// metadata.
type LoadSchemaDocumentFunc func(LoadedSource) (LoadedSchemaDocument, error)

// LoadSchemaDocumentsResult is the source-level result of loading all reachable
// schema sources.
type LoadSchemaDocumentsResult struct {
	ReferenceAliases map[ReferenceKey]string
	SelectedKeys     []string
}

// LoadSchemaDocuments reads initial sources and resolver-discovered references,
// suppressing duplicate source keys and returning the deterministic selected
// compile set.
func LoadSchemaDocuments(sources []Source, maxBytes int64, parse LoadSchemaDocumentFunc) (LoadSchemaDocumentsResult, error) {
	if parse == nil {
		return LoadSchemaDocumentsResult{}, xsderrors.InternalInvariant("schema source loader missing document parser")
	}
	queue := make([]LoadRequest, 0, len(sources))
	for _, source := range sources {
		queue = append(queue, LoadRequest{Source: source})
	}
	var result LoadSchemaDocumentsResult
	loaded := make(map[string]bool)
	docs := make([]LoadedDocument, 0, len(sources))
	for len(queue) != 0 {
		item := queue[0]
		queue = queue[1:]
		next, doc, ok, err := readSchemaLoad(item, maxBytes, loaded, parse)
		if err != nil {
			return LoadSchemaDocumentsResult{}, err
		}
		if !ok {
			continue
		}
		docs = append(docs, LoadedDocument{
			Key:             doc.Key,
			TargetNamespace: doc.Parsed.TargetNamespace,
			Data:            doc.Data,
		})
		if len(doc.ReferenceAliases) != 0 {
			if result.ReferenceAliases == nil {
				result.ReferenceAliases = make(map[ReferenceKey]string)
			}
			maps.Copy(result.ReferenceAliases, doc.ReferenceAliases)
		}
		queue = append(queue, next...)
	}
	result.SelectedKeys = SelectedLoadedDocumentKeys(docs)
	return result, nil
}

type loadedSchemaDocument struct {
	Parsed           LoadedSchemaDocument
	ReferenceAliases map[ReferenceKey]string
	Name             string
	Key              string
	Data             []byte
}

func readSchemaLoad(item LoadRequest, maxBytes int64, loaded map[string]bool, parse LoadSchemaDocumentFunc) ([]LoadRequest, loadedSchemaDocument, bool, error) {
	s := item.Source
	name := s.Name()
	if issue := CheckSourceName(name); issue != SourceNameOK {
		return nil, loadedSchemaDocument{}, false, xsderrors.SchemaCompile(issue.Code(), issue.Message())
	}
	key := Key(name)
	if loaded[key] {
		return nil, loadedSchemaDocument{}, false, nil
	}
	data, err := s.Read(maxBytes)
	if err != nil {
		if item.OptionalMissing && (errors.Is(err, xsderrors.ErrSchemaNotFound) || os.IsNotExist(err)) {
			return nil, loadedSchemaDocument{}, false, nil
		}
		if IsSchemaLimitError(err) {
			return nil, loadedSchemaDocument{}, false, err
		}
		return nil, loadedSchemaDocument{}, false, xsderrors.SchemaParse(xsderrors.CodeSchemaRead, 0, 0, "read schema "+name, err)
	}
	parsed, err := parse(LoadedSource{Name: name, Key: key, Data: data})
	if err != nil {
		return nil, loadedSchemaDocument{}, false, err
	}
	loaded[key] = true
	if s.Resolver() == nil {
		return nil, loadedSchemaDocument{Parsed: parsed, Name: name, Key: key, Data: data}, true, nil
	}
	queue, aliases, err := ResolveSchemaReferences(s, key, parsed.References)
	if err != nil {
		if resolveErr, ok := errors.AsType[*ResolveReferenceError](err); ok {
			return nil, loadedSchemaDocument{}, false, xsderrors.SchemaParse(xsderrors.CodeSchemaRead, 0, 0, "resolve schema "+resolveErr.Location, resolveErr.Err)
		}
		return nil, loadedSchemaDocument{}, false, err
	}
	return queue, loadedSchemaDocument{
		Parsed:           parsed,
		ReferenceAliases: aliases,
		Name:             name,
		Key:              key,
		Data:             data,
	}, true, nil
}

// ResolveReferenceError reports a resolver failure for one schemaLocation.
type ResolveReferenceError struct {
	Err      error
	Location string
}

func (e *ResolveReferenceError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return "resolve schema " + e.Location
	}
	return "resolve schema " + e.Location + ": " + e.Err.Error()
}

func (e *ResolveReferenceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ResolveSchemaReferences resolves include/import schemaLocation references
// through source's resolver and returns load requests plus resolver aliases.
func ResolveSchemaReferences(source Source, baseKey string, refs []SchemaDocumentReference) ([]LoadRequest, map[ReferenceKey]string, error) {
	resolver := source.Resolver()
	if resolver == nil {
		return nil, nil, nil
	}
	var loads []LoadRequest
	var aliases map[ReferenceKey]string
	for _, ref := range refs {
		if ref.Namespace == vocab.XMLNamespaceURI {
			continue
		}
		next, err := resolver.ResolveSchema(source.Name(), ref.Location)
		if err != nil {
			if errors.Is(err, xsderrors.ErrSchemaNotFound) {
				continue
			}
			return nil, nil, &ResolveReferenceError{Location: ref.Location, Err: err}
		}
		if next.Resolver() == nil {
			next = next.WithResolver(resolver)
		}
		if next.Name() != "" {
			if aliases == nil {
				aliases = make(map[ReferenceKey]string)
			}
			aliases[ReferenceKey{Base: baseKey, Location: ref.Location}] = Key(next.Name())
		}
		loads = append(loads, LoadRequest{Source: next, OptionalMissing: true})
	}
	return loads, aliases, nil
}

// TargetNamespaceIssue identifies an invalid loaded schema reference target.
type TargetNamespaceIssue uint8

const (
	// TargetNamespaceOK reports a valid referenced schema target namespace.
	TargetNamespaceOK TargetNamespaceIssue = iota
	// ImportTargetNamespaceMismatch reports an imported target mismatch.
	ImportTargetNamespaceMismatch
	// IncludeTargetNamespaceMismatch reports an included target mismatch.
	IncludeTargetNamespaceMismatch
)

// Code returns the schema compile error code for issue.
func (i TargetNamespaceIssue) Code() xsderrors.Code {
	switch i {
	case ImportTargetNamespaceMismatch, IncludeTargetNamespaceMismatch:
		return xsderrors.CodeSchemaReference
	default:
		return ""
	}
}

// Message returns the schema compile diagnostic text for issue.
func (i TargetNamespaceIssue) Message() string {
	switch i {
	case ImportTargetNamespaceMismatch:
		return "import namespace does not match imported schema targetNamespace"
	case IncludeTargetNamespaceMismatch:
		return "included schema targetNamespace does not match including schema"
	default:
		return ""
	}
}

// CheckImportedTargetNamespace validates the target namespace of a resolved
// xs:import document against the import namespace.
func CheckImportedTargetNamespace(namespace, referencedTarget string) TargetNamespaceIssue {
	if namespace != referencedTarget {
		return ImportTargetNamespaceMismatch
	}
	return TargetNamespaceOK
}

// CheckIncludedTargetNamespace validates the target namespace of a resolved
// xs:include document against the including schema target namespace. Empty
// referencedTarget is allowed for chameleon include adoption.
func CheckIncludedTargetNamespace(target, referencedTarget string) TargetNamespaceIssue {
	if referencedTarget != "" && referencedTarget != target {
		return IncludeTargetNamespaceMismatch
	}
	return TargetNamespaceOK
}

// chameleonAdoptionInput is the source reference state needed to decide whether
// a chameleon include should be adopted or cloned for a target namespace.
type chameleonAdoptionInput struct {
	CloneExists               func(string) bool
	TargetNamespace           string
	ReferencedTargetNamespace string
	ResolvedKey               string
	ExistingTargetNamespace   string
}

// chameleonAdoptionAction identifies the caller work needed for a chameleon
// include.
type chameleonAdoptionAction uint8

const (
	// chameleonNoAdoption reports that no adoption or clone is needed.
	chameleonNoAdoption chameleonAdoptionAction = iota
	// chameleonAdoptSource reports that the resolved source should be adopted by
	// the input target namespace.
	chameleonAdoptSource
	// chameleonCloneSource reports that the resolved source is already adopted by
	// another namespace and needs a per-namespace clone.
	chameleonCloneSource
)

// chameleonAdoption is the source policy decision for a chameleon include.
type chameleonAdoption struct {
	CloneKey string
	Action   chameleonAdoptionAction
	Issue    TargetNamespaceIssue
}

// checkChameleonIncludeAdoption validates include target namespaces and decides
// whether a target namespace must adopt or clone a chameleon schema source.
func checkChameleonIncludeAdoption(in chameleonAdoptionInput) chameleonAdoption {
	if issue := CheckIncludedTargetNamespace(in.TargetNamespace, in.ReferencedTargetNamespace); issue != TargetNamespaceOK {
		return chameleonAdoption{Issue: issue}
	}
	if in.TargetNamespace == "" || in.ReferencedTargetNamespace != "" {
		return chameleonAdoption{}
	}
	if in.ExistingTargetNamespace == "" {
		return chameleonAdoption{Action: chameleonAdoptSource}
	}
	if in.ExistingTargetNamespace == in.TargetNamespace {
		return chameleonAdoption{}
	}
	cloneKey := chameleonCloneKey(in.ResolvedKey, in.TargetNamespace)
	if in.CloneExists != nil && in.CloneExists(cloneKey) {
		return chameleonAdoption{}
	}
	return chameleonAdoption{Action: chameleonCloneSource, CloneKey: cloneKey}
}

func chameleonCloneKey(resolvedKey, target string) string {
	return resolvedKey + "\x00" + target
}

// ChameleonDocument is the source reference projection of one parsed schema
// document participating in chameleon include propagation.
type ChameleonDocument struct {
	Key             string
	Name            string
	TargetNamespace string
	References      []SchemaDocumentReference
	Loaded          bool
	// LoadedOnly makes the document available for schemaLocation resolution
	// without processing its includes as part of the selected compile set.
	LoadedOnly bool
}

// ChameleonClone records one synthetic chameleon document clone required for
// a target namespace.
type ChameleonClone struct {
	SourceKey       string
	CloneKey        string
	TargetNamespace string
}

// ChameleonPlan is one propagation pass worth of source state changes.
type ChameleonPlan struct {
	AdoptedTargets   map[string]string
	ReferenceAliases map[ReferenceKey]string
	Clones           []ChameleonClone
}

// Changed reports whether the plan contains source state updates.
func (p ChameleonPlan) Changed() bool {
	return len(p.AdoptedTargets) != 0 || len(p.ReferenceAliases) != 0 || len(p.Clones) != 0
}

// ChameleonPropagationIssue reports a source reference rule violation found
// during chameleon include propagation.
type ChameleonPropagationIssue struct {
	DocumentKey string
	Location    string
	Line        int
	Column      int
	Issue       TargetNamespaceIssue
}

// PlanChameleonIncludes plans one chameleon include propagation pass. The
// caller applies the returned state and repeats until Changed reports false.
func PlanChameleonIncludes(docs []ChameleonDocument, resolvedRefs map[ReferenceKey]string, adoptedTargets map[string]string) (ChameleonPlan, ChameleonPropagationIssue) {
	loaded := loadedChameleonDocuments(docs)
	if len(loaded) == 0 {
		return ChameleonPlan{}, ChameleonPropagationIssue{}
	}
	adopted := maps.Clone(adoptedTargets)
	if adopted == nil {
		adopted = make(map[string]string)
	}
	var plan ChameleonPlan
	for _, doc := range docs {
		if doc.LoadedOnly {
			continue
		}
		target := chameleonDocumentTarget(doc, adopted)
		for _, ref := range doc.References {
			if ref.Kind != SchemaReferenceInclude {
				continue
			}
			resolved, ok := ResolveLoadedSchemaLocation(doc.Name, doc.Key, ref.Location, resolvedRefs, func(key string) bool {
				_, loaded := loaded[key]
				return loaded
			})
			if !ok {
				continue
			}
			referenced := loaded[resolved]
			adoption := checkChameleonIncludeAdoption(chameleonAdoptionInput{
				CloneExists: func(key string) bool {
					_, ok := adopted[key]
					return ok
				},
				TargetNamespace:           target,
				ReferencedTargetNamespace: referenced.TargetNamespace,
				ResolvedKey:               resolved,
				ExistingTargetNamespace:   adopted[resolved],
			})
			if adoption.Issue != TargetNamespaceOK {
				return plan, ChameleonPropagationIssue{
					Issue:       adoption.Issue,
					DocumentKey: doc.Key,
					Location:    ref.Location,
					Line:        ref.Line,
					Column:      ref.Column,
				}
			}
			switch adoption.Action {
			case chameleonNoAdoption:
				continue
			case chameleonAdoptSource:
				setChameleonAdoptedTarget(&plan, adopted, resolved, target)
			case chameleonCloneSource:
				setChameleonAdoptedTarget(&plan, adopted, adoption.CloneKey, target)
				plan.Clones = append(plan.Clones, ChameleonClone{
					SourceKey:       resolved,
					CloneKey:        adoption.CloneKey,
					TargetNamespace: target,
				})
				copyReferenceAliases(&plan, chameleonCloneReferenceAliases(adoption.CloneKey, chameleonCloneReferences(referenced, resolvedRefs, loaded)))
			}
		}
	}
	return plan, ChameleonPropagationIssue{}
}

func loadedChameleonDocuments(docs []ChameleonDocument) map[string]ChameleonDocument {
	loaded := make(map[string]ChameleonDocument)
	for _, doc := range docs {
		if doc.Loaded {
			loaded[doc.Key] = doc
		}
	}
	return loaded
}

func chameleonDocumentTarget(doc ChameleonDocument, adopted map[string]string) string {
	if doc.TargetNamespace != "" {
		return doc.TargetNamespace
	}
	return adopted[doc.Key]
}

func setChameleonAdoptedTarget(plan *ChameleonPlan, adopted map[string]string, key, target string) {
	if plan.AdoptedTargets == nil {
		plan.AdoptedTargets = make(map[string]string)
	}
	adopted[key] = target
	plan.AdoptedTargets[key] = target
}

func chameleonCloneReferences(doc ChameleonDocument, resolvedRefs map[ReferenceKey]string, loaded map[string]ChameleonDocument) []resolvedReference {
	refs := make([]resolvedReference, 0, len(doc.References))
	for _, ref := range doc.References {
		resolved, ok := ResolveLoadedSchemaLocation(doc.Name, doc.Key, ref.Location, resolvedRefs, func(key string) bool {
			_, loaded := loaded[key]
			return loaded
		})
		if ok {
			refs = append(refs, resolvedReference{Location: ref.Location, Key: resolved})
		}
	}
	return refs
}

func copyReferenceAliases(plan *ChameleonPlan, aliases map[ReferenceKey]string) {
	if len(aliases) == 0 {
		return
	}
	if plan.ReferenceAliases == nil {
		plan.ReferenceAliases = make(map[ReferenceKey]string, len(aliases))
	}
	maps.Copy(plan.ReferenceAliases, aliases)
}

// resolvedReference records a schemaLocation reference after it has been
// resolved to a loaded source key.
type resolvedReference struct {
	Location string
	Key      string
}

// chameleonCloneReferenceAliases returns resolver aliases that make references
// from a synthetic chameleon clone key resolve like the original document.
func chameleonCloneReferenceAliases(cloneKey string, refs []resolvedReference) map[ReferenceKey]string {
	if len(refs) == 0 {
		return nil
	}
	aliases := make(map[ReferenceKey]string, len(refs))
	for _, ref := range refs {
		if ref.Location == "" || ref.Key == "" {
			continue
		}
		aliases[ReferenceKey{Base: cloneKey, Location: ref.Location}] = ref.Key
	}
	if len(aliases) == 0 {
		return nil
	}
	return aliases
}

// ReferenceNamespaces identifies namespaces visible to schema QName references.
type ReferenceNamespaces struct {
	Imports         map[string]bool
	TargetNamespace string
	AdoptedTarget   bool
}

// ReferenceNamespaceIssue identifies an invalid schema QName reference namespace.
type ReferenceNamespaceIssue uint8

const (
	// ReferenceNamespaceOK reports a visible reference namespace.
	ReferenceNamespaceOK ReferenceNamespaceIssue = iota
	// ReferenceNamespaceNotImported reports a reference to a namespace not
	// declared by the enclosing schema's imports.
	ReferenceNamespaceNotImported
)

// Code returns the schema compile error code for issue.
func (i ReferenceNamespaceIssue) Code() xsderrors.Code {
	switch i {
	case ReferenceNamespaceNotImported:
		return xsderrors.CodeSchemaReference
	default:
		return ""
	}
}

// Message returns the schema compile diagnostic text for issue.
func (i ReferenceNamespaceIssue) Message(namespace string) string {
	switch i {
	case ReferenceNamespaceNotImported:
		return "namespace is not imported: " + namespace
	default:
		return ""
	}
}

// CheckReferenceNamespace applies chameleon namespace adoption and validates
// that a schema QName reference uses a namespace visible to the source document.
func CheckReferenceNamespace(namespace string, visible ReferenceNamespaces) (string, ReferenceNamespaceIssue) {
	if namespace == "" && visible.TargetNamespace != "" && visible.AdoptedTarget {
		namespace = visible.TargetNamespace
	}
	if referenceNamespaceVisible(namespace, visible) {
		return namespace, ReferenceNamespaceOK
	}
	return namespace, ReferenceNamespaceNotImported
}

func referenceNamespaceVisible(namespace string, visible ReferenceNamespaces) bool {
	if namespace == vocab.XSDNamespaceURI || namespace == vocab.XMLNamespaceURI {
		return true
	}
	if namespace == visible.TargetNamespace {
		return true
	}
	return visible.Imports != nil && visible.Imports[namespace]
}

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
func File(path string) Source {
	path = filepath.Clean(path)
	return Source{
		name: path,
		open: func() (io.ReadCloser, error) {
			return os.Open(path)
		},
		resolver: Resolver(resolveFileSchemaSource),
	}
}

// Reader reads r into an in-memory schema source.
func Reader(name string, r io.Reader) Source {
	return LimitedReader(name, r, math.MaxInt64)
}

// LimitedReader reads at most maxBytes from r into an in-memory schema source.
func LimitedReader(name string, r io.Reader, maxBytes int64) Source {
	if r == nil {
		return Source{name: name, err: ErrNilReader}
	}
	data, err := readLimitedSchemaSource(name, r, maxBytes)
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

// Empty returns a named source with no data or opener.
func Empty(name string) Source {
	return Source{name: name}
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

// Resolver returns the source resolver.
func (s Source) Resolver() Resolver {
	return s.resolver
}

// Read returns a copy of the source bytes.
func (s Source) Read(maxBytes int64) ([]byte, error) {
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
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source has no data or opener")
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
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema reader byte limit must be positive")
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
	msg := "schema source exceeds MaxSchemaSourceBytes"
	if name != "" {
		msg = "schema source " + name + " exceeds MaxSchemaSourceBytes"
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, msg)
}

// IsSchemaLimitError reports whether err is a schema source byte-limit
// diagnostic.
func IsSchemaLimitError(err error) bool {
	x, ok := errors.AsType[*xsderrors.Error](err)
	return ok && x.Code == xsderrors.CodeSchemaLimit
}

func resolveFileSchemaSource(base, location string) (Source, error) {
	path, ok := ResolveLocalSchemaLocation(base, location)
	if !ok {
		return Source{}, xsderrors.ErrSchemaNotFound
	}
	return File(path), nil
}

// Key canonicalizes a schema source name for loaded-document identity.
func Key(name string) string {
	if filepath.VolumeName(name) != "" {
		return filepath.Clean(name)
	}
	u, err := url.Parse(name)
	if err == nil && u.Scheme != "" {
		if path, ok := LocalFileURIPath(u); ok {
			return path
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

// ReferenceKey identifies a schemaLocation reference from one loaded source.
type ReferenceKey struct {
	Base     string
	Location string
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

// ResolveLoadedSchemaLocation resolves a schemaLocation to an already loaded
// source key, preferring resolver-returned aliases over location candidates.
func ResolveLoadedSchemaLocation(baseName, baseKey, location string, resolved map[ReferenceKey]string, loaded func(string) bool) (string, bool) {
	if loaded == nil {
		return "", false
	}
	loc, ok := NormalizeSchemaLocation(location)
	if !ok {
		return "", false
	}
	if resolvedKey, ok := resolved[ReferenceKey{Base: baseKey, Location: loc}]; ok && loaded(resolvedKey) {
		return resolvedKey, true
	}
	for _, key := range LocationKeys(baseName, baseKey, loc) {
		if loaded(key) {
			return key, true
		}
	}
	return "", false
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
	path, err := url.PathUnescape(u.Path)
	if err != nil || path == "" {
		return "", false
	}
	return filepath.Clean(path), true
}

// LoadedDocument is the source identity data needed to decide which parsed
// schema documents participate in compilation.
type LoadedDocument struct {
	Key             string
	TargetNamespace string
	Data            []byte
}

// SelectedLoadedDocumentKeys returns loaded source keys in deterministic key
// order, dropping byte-identical documents only when they share a non-empty
// target namespace that appears more than once.
func SelectedLoadedDocumentKeys(docs []LoadedDocument) []string {
	if len(docs) == 0 {
		return nil
	}
	if len(docs) == 1 {
		return []string{docs[0].Key}
	}
	ordered := slices.Clone(docs)
	slices.SortFunc(ordered, func(a, b LoadedDocument) int {
		return cmp.Compare(a.Key, b.Key)
	})
	targetCounts, hasDuplicateTargets := targetNamespaceCounts(ordered)
	if !hasDuplicateTargets {
		keys := make([]string, len(ordered))
		for i, doc := range ordered {
			keys[i] = doc.Key
		}
		return keys
	}
	keys := make([]string, 0, len(ordered))
	seed := maphash.MakeSeed()
	seenContent := make(map[loadedContentKey][][]byte)
	for _, doc := range ordered {
		if doc.TargetNamespace != "" && targetCounts[doc.TargetNamespace] > 1 {
			key := loadedContentKey{
				target: doc.TargetNamespace,
				size:   len(doc.Data),
				hash:   maphash.Bytes(seed, doc.Data),
			}
			if contentSeen(seenContent[key], doc.Data) {
				continue
			}
			seenContent[key] = append(seenContent[key], doc.Data)
		}
		keys = append(keys, doc.Key)
	}
	return keys
}

func targetNamespaceCounts(docs []LoadedDocument) (map[string]int, bool) {
	if len(docs) < 2 {
		return nil, false
	}
	counts := make(map[string]int, len(docs))
	hasDuplicate := false
	for _, doc := range docs {
		if doc.TargetNamespace == "" {
			continue
		}
		counts[doc.TargetNamespace]++
		if counts[doc.TargetNamespace] == 2 {
			hasDuplicate = true
		}
	}
	return counts, hasDuplicate
}

type loadedContentKey struct {
	target string
	size   int
	hash   uint64
}

func contentSeen(bucket [][]byte, data []byte) bool {
	for _, item := range bucket {
		if bytes.Equal(item, data) {
			return true
		}
	}
	return false
}
