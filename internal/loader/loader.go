package loader

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/types"
)

// Config holds configuration for the schema loader
type Config struct {
	FS fs.FS

	Resolver Resolver

	AllowMissingImportLocations bool
}

type loadKey struct {
	systemID string
	etn      types.NamespaceURI
}

type loadState struct {
	entries map[loadKey]*schemaEntry
}

func newLoadState() loadState {
	return loadState{
		entries: make(map[loadKey]*schemaEntry),
	}
}

type schemaLoadState int

const (
	schemaStateUnknown schemaLoadState = iota
	schemaStateLoading
	schemaStateLoaded
)

type schemaEntry struct {
	schema              *parser.Schema
	pendingDirectives   []pendingDirective
	state               schemaLoadState
	pendingCount        int
	validationRequested bool
	validated           bool
}

func (s *loadState) entry(key loadKey) (*schemaEntry, bool) {
	entry, ok := s.entries[key]
	return entry, ok
}

func (s *loadState) ensureEntry(key loadKey) *schemaEntry {
	if entry, ok := s.entries[key]; ok {
		return entry
	}
	entry := &schemaEntry{}
	s.entries[key] = entry
	return entry
}

func (s *loadState) deleteEntry(key loadKey) {
	delete(s.entries, key)
}

func (s *loadState) loadedSchema(key loadKey) (*parser.Schema, bool) {
	entry, ok := s.entries[key]
	if !ok || entry.state != schemaStateLoaded || entry.schema == nil {
		return nil, false
	}
	return entry.schema, true
}

func (s *loadState) isLoading(key loadKey) bool {
	entry, ok := s.entries[key]
	return ok && entry.state == schemaStateLoading
}

func (s *loadState) loadingSchema(key loadKey) (*parser.Schema, bool) {
	entry, ok := s.entries[key]
	if !ok || entry.state != schemaStateLoading {
		return nil, false
	}
	return entry.schema, true
}

func (s *loadState) schemaForKey(key loadKey) *parser.Schema {
	entry, ok := s.entries[key]
	if !ok {
		return nil
	}
	return entry.schema
}

type pendingDirective struct {
	targetKey         loadKey
	schemaLocation    string
	expectedNamespace string
	kind              parser.DirectiveKind
}

type importTracker struct {
	mergedIncludes map[loadKey]map[loadKey]bool
	mergedImports  map[loadKey]map[loadKey]bool
}

func newImportTracker() importTracker {
	return importTracker{
		mergedIncludes: make(map[loadKey]map[loadKey]bool),
		mergedImports:  make(map[loadKey]map[loadKey]bool),
	}
}

func (t *importTracker) alreadyMergedInclude(baseKey, includeKey loadKey) bool {
	merged, ok := t.mergedIncludes[baseKey]
	if !ok {
		return false
	}
	return merged[includeKey]
}

func (t *importTracker) markMergedInclude(baseKey, includeKey loadKey) {
	if t.mergedIncludes[baseKey] == nil {
		t.mergedIncludes[baseKey] = make(map[loadKey]bool)
	}
	t.mergedIncludes[baseKey][includeKey] = true
}

func (t *importTracker) alreadyMergedImport(baseKey, importKey loadKey) bool {
	merged, ok := t.mergedImports[baseKey]
	if !ok {
		return false
	}
	return merged[importKey]
}

func (t *importTracker) markMergedImport(baseKey, importKey loadKey) {
	if t.mergedImports[baseKey] == nil {
		t.mergedImports[baseKey] = make(map[loadKey]bool)
	}
	t.mergedImports[baseKey][importKey] = true
}

type validationMode int

const (
	validateSchema validationMode = iota
	skipSchemaValidation
)

// SchemaLoader loads XML schemas with import/include resolution
type SchemaLoader struct {
	imports  importTracker
	resolver Resolver
	state    loadState
	config   Config
}

// NewLoader creates a new schema loader with the given configuration
func NewLoader(cfg Config) *SchemaLoader {
	res := cfg.Resolver
	if res == nil && cfg.FS != nil {
		res = NewFSResolver(cfg.FS)
	}
	return &SchemaLoader{
		config:   cfg,
		state:    newLoadState(),
		imports:  newImportTracker(),
		resolver: res,
	}
}

// Load loads a schema from the given location and validates it.
func (l *SchemaLoader) Load(location string) (*parser.Schema, error) {
	if l == nil || l.resolver == nil {
		return nil, fmt.Errorf("no resolver configured")
	}
	return l.loadRoot(location, validateSchema)
}

func (l *SchemaLoader) loadKey(systemID string, etn types.NamespaceURI) loadKey {
	return loadKey{systemID: systemID, etn: etn}
}

// loadRoot loads the root schema by resolving the provided location.
func (l *SchemaLoader) loadRoot(location string, mode validationMode) (*parser.Schema, error) {
	doc, systemID, err := l.resolve(ResolveRequest{
		BaseSystemID:   "",
		SchemaLocation: location,
		Kind:           ResolveInclude,
	})
	if err != nil {
		return nil, err
	}
	result, err := parseSchemaDocument(doc, systemID)
	if err != nil {
		return nil, err
	}
	key := l.loadKey(systemID, result.Schema.TargetNamespace)
	return l.loadParsed(result, systemID, key, mode)
}

// loadResolved loads a schema from an already-resolved reader and systemID.
func (l *SchemaLoader) loadResolved(doc io.ReadCloser, systemID string, key loadKey, mode validationMode) (*parser.Schema, error) {
	session := newLoadSession(l, systemID, key, doc)

	if schema, ok := l.state.loadedSchema(key); ok {
		if mode == validateSchema {
			entry := l.state.ensureEntry(key)
			entry.validationRequested = true
			if resolveErr := l.resolvePendingImportsFor(key); resolveErr != nil {
				return nil, resolveErr
			}
		}
		if closeErr := doc.Close(); closeErr != nil {
			return nil, closeErr
		}
		return schema, nil
	}

	loadedSchema, err := session.handleCircularLoad()
	if err != nil || loadedSchema != nil {
		if closeErr := doc.Close(); closeErr != nil && err == nil {
			return nil, closeErr
		}
		return loadedSchema, err
	}

	result, err := session.parseSchema()
	if err != nil {
		return nil, err
	}
	return l.loadParsed(result, systemID, key, mode)
}

func (l *SchemaLoader) loadParsed(result *parser.ParseResult, systemID string, key loadKey, mode validationMode) (*parser.Schema, error) {
	if schema, ok := l.state.loadedSchema(key); ok {
		if mode == validateSchema {
			entry := l.state.ensureEntry(key)
			entry.validationRequested = true
			if resolveErr := l.resolvePendingImportsFor(key); resolveErr != nil {
				return nil, resolveErr
			}
		}
		return schema, nil
	}

	if l.state.isLoading(key) {
		if inProgress, ok := l.state.loadingSchema(key); ok && inProgress != nil {
			return inProgress, nil
		}
		return nil, fmt.Errorf("circular dependency detected: %s", systemID)
	}

	entry := l.state.ensureEntry(key)
	entry.state = schemaStateLoading
	entry.schema = nil
	defer func() {
		if entry.state != schemaStateLoading {
			return
		}
		entry.state = schemaStateUnknown
		entry.schema = nil
		if entry.pendingCount == 0 && len(entry.pendingDirectives) == 0 && !entry.validationRequested && !entry.validated {
			l.state.deleteEntry(key)
		}
	}()

	schema := result.Schema
	initSchemaOrigins(schema, systemID)
	entry.schema = schema
	registerImports(schema, result.Imports)

	if validateErr := validateImportConstraints(schema, result.Imports); validateErr != nil {
		return nil, validateErr
	}

	session := newLoadSession(l, systemID, key, nil)
	if directivesErr := session.processDirectives(schema, result.Directives); directivesErr != nil {
		return nil, directivesErr
	}

	if mode == validateSchema {
		entry.validationRequested = true
	}

	entry.schema = schema
	entry.state = schemaStateLoaded

	if err := l.resolvePendingImportsFor(key); err != nil {
		return nil, err
	}

	return schema, nil
}

func (l *SchemaLoader) resolve(req ResolveRequest) (io.ReadCloser, string, error) {
	if l == nil || l.resolver == nil {
		return nil, "", fmt.Errorf("no resolver configured")
	}
	return l.resolver.Resolve(req)
}

func (l *SchemaLoader) validateLoadedSchema(schema *parser.Schema) error {
	if err := l.resolveGroupReferences(schema); err != nil {
		return fmt.Errorf("resolve group references: %w", err)
	}

	// phase 2: Resolve all type references (two-phase resolution)
	if err := resolver.ResolveTypeReferences(schema); err != nil {
		return fmt.Errorf("resolve type references: %w", err)
	}

	if err := validateSchemaConstraints(schema); err != nil {
		return err
	}

	return nil
}

// LoadResolved loads a schema from a resolved reader and systemID, then validates it.
func (l *SchemaLoader) LoadResolved(doc io.ReadCloser, systemID string) (*parser.Schema, error) {
	if l == nil {
		return nil, fmt.Errorf("no loader configured")
	}
	if systemID == "" {
		return nil, fmt.Errorf("missing systemID")
	}
	result, err := parseSchemaDocument(doc, systemID)
	if err != nil {
		return nil, err
	}
	key := l.loadKey(systemID, result.Schema.TargetNamespace)
	return l.loadParsed(result, systemID, key, validateSchema)
}

// GetLoaded returns a loaded schema by systemID and effective target namespace.
func (l *SchemaLoader) GetLoaded(systemID string, etn types.NamespaceURI) (*parser.Schema, bool) {
	key := l.loadKey(systemID, etn)
	schema, ok := l.state.loadedSchema(key)
	return schema, ok
}
func (l *SchemaLoader) alreadyMergedInclude(baseKey, includeKey loadKey) bool {
	return l.imports.alreadyMergedInclude(baseKey, includeKey)
}

func (l *SchemaLoader) markMergedInclude(baseKey, includeKey loadKey) {
	l.imports.markMergedInclude(baseKey, includeKey)
}

func (l *SchemaLoader) alreadyMergedImport(baseKey, importKey loadKey) bool {
	return l.imports.alreadyMergedImport(baseKey, importKey)
}

func (l *SchemaLoader) markMergedImport(baseKey, importKey loadKey) {
	l.imports.markMergedImport(baseKey, importKey)
}

func (l *SchemaLoader) deferImport(sourceKey, targetKey loadKey, schemaLocation, expectedNamespace string) {
	sourceEntry := l.state.ensureEntry(sourceKey)
	for _, pending := range sourceEntry.pendingDirectives {
		if pending.kind == parser.DirectiveImport && pending.targetKey == targetKey {
			return
		}
	}
	sourceEntry.pendingDirectives = append(sourceEntry.pendingDirectives, pendingDirective{
		kind:              parser.DirectiveImport,
		targetKey:         targetKey,
		schemaLocation:    schemaLocation,
		expectedNamespace: expectedNamespace,
	})
	targetEntry := l.state.ensureEntry(targetKey)
	targetEntry.pendingCount++
}

func (l *SchemaLoader) deferInclude(sourceKey, targetKey loadKey, schemaLocation string) {
	sourceEntry := l.state.ensureEntry(sourceKey)
	for _, pending := range sourceEntry.pendingDirectives {
		if pending.kind == parser.DirectiveInclude && pending.targetKey == targetKey {
			return
		}
	}
	sourceEntry.pendingDirectives = append(sourceEntry.pendingDirectives, pendingDirective{
		kind:           parser.DirectiveInclude,
		targetKey:      targetKey,
		schemaLocation: schemaLocation,
	})
	targetEntry := l.state.ensureEntry(targetKey)
	targetEntry.pendingCount++
}

func (l *SchemaLoader) resolvePendingImportsFor(sourceKey loadKey) error {
	sourceEntry := l.state.ensureEntry(sourceKey)
	if sourceEntry.pendingCount > 0 {
		return nil
	}
	pendingDirectives := sourceEntry.pendingDirectives
	if len(pendingDirectives) == 0 {
		return l.validateIfRequested(sourceKey)
	}
	source := l.schemaForKey(sourceKey)
	if source == nil {
		return fmt.Errorf("pending import source not found: %s", sourceKey.systemID)
	}
	sourceEntry.pendingDirectives = nil

	for _, entry := range pendingDirectives {
		target := l.schemaForKey(entry.targetKey)
		if target == nil {
			return fmt.Errorf("pending directive target not found: %s", entry.targetKey.systemID)
		}
		switch entry.kind {
		case parser.DirectiveInclude:
			includingNS := entry.targetKey.etn
			if !l.isIncludeNamespaceCompatible(includingNS, source.TargetNamespace) {
				return fmt.Errorf("included schema %s has different target namespace: %s != %s",
					entry.schemaLocation, source.TargetNamespace, includingNS)
			}
			remapMode := keepNamespace
			if !includingNS.IsEmpty() && source.TargetNamespace.IsEmpty() {
				remapMode = remapNamespace
			}
			if err := l.mergeSchema(target, source, mergeInclude, remapMode); err != nil {
				return fmt.Errorf("merge included schema %s: %w", entry.schemaLocation, err)
			}
			l.markMergedInclude(entry.targetKey, sourceKey)
		case parser.DirectiveImport:
			if entry.expectedNamespace != "" && source.TargetNamespace != types.NamespaceURI(entry.expectedNamespace) {
				return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
					entry.schemaLocation, entry.expectedNamespace, source.TargetNamespace)
			}
			if entry.expectedNamespace == "" && !source.TargetNamespace.IsEmpty() {
				return fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s",
					entry.schemaLocation, source.TargetNamespace)
			}
			if err := l.mergeSchema(target, source, mergeImport, keepNamespace); err != nil {
				return fmt.Errorf("merge imported schema %s: %w", entry.schemaLocation, err)
			}
			l.markMergedImport(entry.targetKey, sourceKey)
		default:
			return fmt.Errorf("unknown pending directive kind: %d", entry.kind)
		}

		targetEntry := l.state.ensureEntry(entry.targetKey)
		targetEntry.pendingCount--
		if targetEntry.pendingCount < 0 {
			targetEntry.pendingCount = 0
		}
		if targetEntry.pendingCount == 0 {
			if err := l.resolvePendingImportsFor(entry.targetKey); err != nil {
				return err
			}
		}
	}

	return l.validateIfRequested(sourceKey)
}

func (l *SchemaLoader) schemaForKey(key loadKey) *parser.Schema {
	return l.state.schemaForKey(key)
}

func (l *SchemaLoader) validateIfRequested(key loadKey) error {
	entry, ok := l.state.entry(key)
	if !ok || !entry.validationRequested || entry.validated {
		return nil
	}
	if entry.pendingCount > 0 {
		return nil
	}
	schema := l.schemaForKey(key)
	if schema == nil {
		return fmt.Errorf("schema not available for validation: %s", key.systemID)
	}
	if err := l.validateLoadedSchema(schema); err != nil {
		return err
	}
	entry.validated = true
	return nil
}

func ensureNamespaceMap(m map[types.NamespaceURI]map[types.NamespaceURI]bool, key types.NamespaceURI) map[types.NamespaceURI]bool {
	if m[key] == nil {
		m[key] = make(map[types.NamespaceURI]bool)
	}
	return m[key]
}

func registerImports(sch *parser.Schema, imports []parser.ImportInfo) {
	if sch == nil {
		return
	}
	if sch.ImportedNamespaces == nil {
		sch.ImportedNamespaces = make(map[types.NamespaceURI]map[types.NamespaceURI]bool)
	}
	fromNS := sch.TargetNamespace
	imported := ensureNamespaceMap(sch.ImportedNamespaces, fromNS)
	for _, imp := range imports {
		ns := types.NamespaceURI(imp.Namespace)
		imported[ns] = true
	}

	if sch.ImportContexts == nil {
		sch.ImportContexts = make(map[string]parser.ImportContext)
	}
	if sch.Location != "" {
		ctx := sch.ImportContexts[sch.Location]
		if ctx.Imports == nil {
			ctx.Imports = make(map[types.NamespaceURI]bool)
		}
		ctx.TargetNamespace = sch.TargetNamespace
		for _, imp := range imports {
			ns := types.NamespaceURI(imp.Namespace)
			ctx.Imports[ns] = true
		}
		sch.ImportContexts[sch.Location] = ctx
	}
}

func validateImportConstraints(schema *parser.Schema, imports []parser.ImportInfo) error {
	if schema.TargetNamespace.IsEmpty() {
		for _, imp := range imports {
			if imp.Namespace == "" {
				return fmt.Errorf("schema without targetNamespace cannot use import without namespace attribute (namespace attribute is required)")
			}
		}
	}
	for _, imp := range imports {
		if imp.Namespace == "" {
			continue
		}
		if !schema.TargetNamespace.IsEmpty() && types.NamespaceURI(imp.Namespace) == schema.TargetNamespace {
			return fmt.Errorf("import namespace %s must be different from target namespace", imp.Namespace)
		}
	}
	return nil
}

func validateSchemaConstraints(schema *parser.Schema) error {
	validationErrors := ValidateSchema(schema)
	if len(validationErrors) == 0 {
		return nil
	}
	var errMsg strings.Builder
	errMsg.WriteString("schema validation failed:")
	for _, err := range validationErrors {
		errMsg.WriteString("\n  - ")
		errMsg.WriteString(err.Error())
	}
	return errors.New(errMsg.String())
}

func initSchemaOrigins(schema *parser.Schema, location string) {
	if schema == nil {
		return
	}
	schema.Location = parser.ImportContextKey("", location)
	for qname := range schema.ElementDecls {
		if schema.ElementOrigins[qname] == "" {
			schema.ElementOrigins[qname] = schema.Location
		}
	}
	for qname := range schema.TypeDefs {
		if schema.TypeOrigins[qname] == "" {
			schema.TypeOrigins[qname] = schema.Location
		}
	}
	for qname := range schema.AttributeDecls {
		if schema.AttributeOrigins[qname] == "" {
			schema.AttributeOrigins[qname] = schema.Location
		}
	}
	for qname := range schema.AttributeGroups {
		if schema.AttributeGroupOrigins[qname] == "" {
			schema.AttributeGroupOrigins[qname] = schema.Location
		}
	}
	for qname := range schema.Groups {
		if schema.GroupOrigins[qname] == "" {
			schema.GroupOrigins[qname] = schema.Location
		}
	}
	for qname := range schema.NotationDecls {
		if schema.NotationOrigins[qname] == "" {
			schema.NotationOrigins[qname] = schema.Location
		}
	}
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err)
}

// isIncludeNamespaceCompatible checks if target namespaces are compatible for include
// Rules according to XSD 1.0 spec:
// - If including schema has a target namespace, included schema must have the same namespace OR no namespace
// - If including schema has no target namespace, included schema must also have no target namespace
func (l *SchemaLoader) isIncludeNamespaceCompatible(includingNS, includedNS types.NamespaceURI) bool {
	// same namespace - always compatible
	if includingNS == includedNS {
		return true
	}
	// including schema has namespace, included schema has no namespace - compatible
	if !includingNS.IsEmpty() && includedNS.IsEmpty() {
		return true
	}
	// all other cases are incompatible
	return false
}
