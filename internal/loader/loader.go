package loader

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Config holds configuration for the schema loader
type Config struct {
	FS                          fs.FS
	Resolver                    Resolver
	SchemaParseOptions          []xmlstream.Option
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
	includeInserted     []int
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
	includeDeclIndex  int
	includeIndex      int
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

func (t *importTracker) unmarkMergedInclude(baseKey, includeKey loadKey) {
	merged, ok := t.mergedIncludes[baseKey]
	if !ok {
		return
	}
	delete(merged, includeKey)
	if len(merged) == 0 {
		delete(t.mergedIncludes, baseKey)
	}
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

func (t *importTracker) unmarkMergedImport(baseKey, importKey loadKey) {
	merged, ok := t.mergedImports[baseKey]
	if !ok {
		return
	}
	delete(merged, importKey)
	if len(merged) == 0 {
		delete(t.mergedImports, baseKey)
	}
}

type validationMode int

const (
	validateSchema validationMode = iota
	skipSchemaValidation
)

// SchemaLoader loads XML schemas with import/include resolution.
// It is not safe for concurrent use; create one per goroutine or serialize access.
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

func (l *SchemaLoader) cleanupEntryIfUnused(key loadKey) {
	entry, ok := l.state.entry(key)
	if !ok || entry == nil {
		return
	}
	if entry.state != schemaStateUnknown || entry.schema != nil {
		return
	}
	if entry.pendingCount != 0 || len(entry.pendingDirectives) != 0 || entry.validationRequested || entry.validated {
		return
	}
	l.state.deleteEntry(key)
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
	result, err := parseSchemaDocument(doc, systemID, l.config.SchemaParseOptions...)
	if err != nil {
		return nil, err
	}
	key := l.loadKey(systemID, result.Schema.TargetNamespace)
	return l.loadParsed(result, systemID, key, mode)
}

// loadResolved loads a schema from an already-resolved reader and systemID.
func (l *SchemaLoader) loadResolved(doc io.ReadCloser, systemID string, key loadKey, mode validationMode) (*parser.Schema, error) {
	session := newLoadSession(l, systemID, key, doc)

	if sch, ok := l.state.loadedSchema(key); ok {
		if mode == validateSchema {
			entry := l.state.ensureEntry(key)
			entry.validationRequested = true
			if resolveErr := l.resolvePendingImportsFor(key); resolveErr != nil {
				_ = doc.Close()
				return nil, resolveErr
			}
		}
		if closeErr := doc.Close(); closeErr != nil {
			return nil, closeErr
		}
		return sch, nil
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

func (l *SchemaLoader) loadParsed(result *parser.ParseResult, systemID string, key loadKey, mode validationMode) (sch *parser.Schema, err error) {
	if loadedSchema, ok := l.state.loadedSchema(key); ok {
		if mode == validateSchema {
			entry := l.state.ensureEntry(key)
			entry.validationRequested = true
			if resolveErr := l.resolvePendingImportsFor(key); resolveErr != nil {
				return nil, resolveErr
			}
		}
		return loadedSchema, nil
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

	sch = result.Schema
	initSchemaOrigins(sch, systemID)
	entry.schema = sch
	if len(result.Includes) > 0 {
		entry.includeInserted = make([]int, len(result.Includes))
	} else {
		entry.includeInserted = nil
	}
	registerImports(sch, result.Imports)

	if validateErr := validateImportConstraints(sch, result.Imports); validateErr != nil {
		return nil, validateErr
	}

	session := newLoadSession(l, systemID, key, nil)
	defer func() {
		if err != nil {
			session.rollback()
		}
	}()
	if directivesErr := session.processDirectives(sch, result.Directives); directivesErr != nil {
		return nil, directivesErr
	}

	if mode == validateSchema {
		entry.validationRequested = true
	}

	entry.schema = sch
	entry.state = schemaStateLoaded

	if err := l.resolvePendingImportsFor(key); err != nil {
		entry.schema = nil
		entry.state = schemaStateUnknown
		entry.pendingDirectives = nil
		entry.pendingCount = 0
		entry.validationRequested = false
		entry.validated = false
		return nil, err
	}

	return sch, nil
}

func (l *SchemaLoader) resolve(req ResolveRequest) (io.ReadCloser, string, error) {
	if l == nil || l.resolver == nil {
		return nil, "", fmt.Errorf("no resolver configured")
	}
	return l.resolver.Resolve(req)
}

func (l *SchemaLoader) validateLoadedSchema(sch *parser.Schema) error {
	if err := l.resolveGroupReferences(sch); err != nil {
		return fmt.Errorf("resolve group references: %w", err)
	}

	structureErrors := schemacheck.ValidateStructure(sch)
	if len(structureErrors) > 0 {
		return formatSchemaErrors(structureErrors)
	}
	if err := schema.MarkSemantic(sch); err != nil {
		return err
	}

	if err := resolver.ResolveTypeReferences(sch); err != nil {
		return fmt.Errorf("resolve type references: %w", err)
	}

	refErrors := resolver.ValidateReferences(sch)
	if len(refErrors) > 0 {
		return formatSchemaErrors(refErrors)
	}

	parser.UpdatePlaceholderState(sch)
	if err := schema.MarkResolved(sch); err != nil {
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
	result, err := parseSchemaDocument(doc, systemID, l.config.SchemaParseOptions...)
	if err != nil {
		return nil, err
	}
	key := l.loadKey(systemID, result.Schema.TargetNamespace)
	return l.loadParsed(result, systemID, key, validateSchema)
}

// GetLoaded returns a loaded schema by systemID and effective target namespace.
func (l *SchemaLoader) GetLoaded(systemID string, etn types.NamespaceURI) (*parser.Schema, bool) {
	key := l.loadKey(systemID, etn)
	sch, ok := l.state.loadedSchema(key)
	return sch, ok
}
func (l *SchemaLoader) alreadyMergedInclude(baseKey, includeKey loadKey) bool {
	return l.imports.alreadyMergedInclude(baseKey, includeKey)
}

func (l *SchemaLoader) markMergedInclude(baseKey, includeKey loadKey) {
	l.imports.markMergedInclude(baseKey, includeKey)
}

func (l *SchemaLoader) unmarkMergedInclude(baseKey, includeKey loadKey) {
	l.imports.unmarkMergedInclude(baseKey, includeKey)
}

func (l *SchemaLoader) alreadyMergedImport(baseKey, importKey loadKey) bool {
	return l.imports.alreadyMergedImport(baseKey, importKey)
}

func (l *SchemaLoader) markMergedImport(baseKey, importKey loadKey) {
	l.imports.markMergedImport(baseKey, importKey)
}

func (l *SchemaLoader) unmarkMergedImport(baseKey, importKey loadKey) {
	l.imports.unmarkMergedImport(baseKey, importKey)
}

func (l *SchemaLoader) deferImport(sourceKey, targetKey loadKey, schemaLocation, expectedNamespace string) bool {
	sourceEntry := l.state.ensureEntry(sourceKey)
	for _, pending := range sourceEntry.pendingDirectives {
		if pending.kind == parser.DirectiveImport && pending.targetKey == targetKey {
			return false
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
	return true
}

func (l *SchemaLoader) deferInclude(sourceKey, targetKey loadKey, include parser.IncludeInfo) bool {
	sourceEntry := l.state.ensureEntry(sourceKey)
	for _, pending := range sourceEntry.pendingDirectives {
		if pending.kind == parser.DirectiveInclude && pending.targetKey == targetKey {
			return false
		}
	}
	sourceEntry.pendingDirectives = append(sourceEntry.pendingDirectives, pendingDirective{
		kind:             parser.DirectiveInclude,
		targetKey:        targetKey,
		schemaLocation:   include.SchemaLocation,
		includeDeclIndex: include.DeclIndex,
		includeIndex:     include.IncludeIndex,
	})
	targetEntry := l.state.ensureEntry(targetKey)
	targetEntry.pendingCount++
	return true
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

	type stagedTarget struct {
		schema *parser.Schema
		entry  *schemaEntry
	}
	staged := make(map[loadKey]*stagedTarget, len(pendingDirectives))

	stageTarget := func(targetKey loadKey) (*stagedTarget, error) {
		if existing, ok := staged[targetKey]; ok {
			return existing, nil
		}
		target := l.schemaForKey(targetKey)
		if target == nil {
			return nil, fmt.Errorf("pending directive target not found: %s", targetKey.systemID)
		}
		entry, ok := l.state.entry(targetKey)
		if !ok || entry == nil {
			return nil, fmt.Errorf("pending directive tracking missing for %s", targetKey.systemID)
		}
		stagedEntry := &schemaEntry{}
		if len(entry.includeInserted) > 0 {
			stagedEntry.includeInserted = append([]int(nil), entry.includeInserted...)
		}
		stagedTarget := &stagedTarget{
			schema: cloneSchemaForMerge(target),
			entry:  stagedEntry,
		}
		staged[targetKey] = stagedTarget
		return stagedTarget, nil
	}

	for _, entry := range pendingDirectives {
		target, err := stageTarget(entry.targetKey)
		if err != nil {
			return err
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
			includeInfo := parser.IncludeInfo{
				SchemaLocation: entry.schemaLocation,
				DeclIndex:      entry.includeDeclIndex,
				IncludeIndex:   entry.includeIndex,
			}
			insertAt, err := includeInsertIndex(target.entry, includeInfo, len(target.schema.GlobalDecls))
			if err != nil {
				return err
			}
			beforeLen := len(target.schema.GlobalDecls)
			if err := l.mergeSchema(target.schema, source, mergeInclude, remapMode, insertAt); err != nil {
				return fmt.Errorf("merge included schema %s: %w", entry.schemaLocation, err)
			}
			inserted := len(target.schema.GlobalDecls) - beforeLen
			if err := recordIncludeInserted(target.entry, entry.includeIndex, inserted); err != nil {
				return err
			}
		case parser.DirectiveImport:
			if entry.expectedNamespace != "" && source.TargetNamespace != types.NamespaceURI(entry.expectedNamespace) {
				return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
					entry.schemaLocation, entry.expectedNamespace, source.TargetNamespace)
			}
			if entry.expectedNamespace == "" && !source.TargetNamespace.IsEmpty() {
				return fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s",
					entry.schemaLocation, source.TargetNamespace)
			}
			if err := l.mergeSchema(target.schema, source, mergeImport, keepNamespace, len(target.schema.GlobalDecls)); err != nil {
				return fmt.Errorf("merge imported schema %s: %w", entry.schemaLocation, err)
			}
		default:
			return fmt.Errorf("unknown pending directive kind: %d", entry.kind)
		}
	}

	for key, stagedTarget := range staged {
		target := l.schemaForKey(key)
		if target == nil {
			return fmt.Errorf("pending directive target not found: %s", key.systemID)
		}
		*target = *stagedTarget.schema
		if entry, ok := l.state.entry(key); ok && entry != nil {
			entry.includeInserted = stagedTarget.entry.includeInserted
		}
	}

	for _, entry := range pendingDirectives {
		switch entry.kind {
		case parser.DirectiveInclude:
			l.markMergedInclude(entry.targetKey, sourceKey)
		case parser.DirectiveImport:
			l.markMergedImport(entry.targetKey, sourceKey)
		}
	}

	sourceEntry.pendingDirectives = nil

	for _, entry := range pendingDirectives {
		if err := l.decrementPendingAndResolve(entry.targetKey); err != nil {
			return err
		}
	}

	return l.validateIfRequested(sourceKey)
}

func (l *SchemaLoader) decrementPendingAndResolve(targetKey loadKey) error {
	targetEntry := l.state.ensureEntry(targetKey)
	if targetEntry.pendingCount == 0 {
		return fmt.Errorf("pending directive count underflow for %s", targetKey.systemID)
	}
	targetEntry.pendingCount--
	if targetEntry.pendingCount == 0 {
		if err := l.resolvePendingImportsFor(targetKey); err != nil {
			return err
		}
	}
	return nil
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
	sch := l.schemaForKey(key)
	if sch == nil {
		return fmt.Errorf("schema not available for validation: %s", key.systemID)
	}
	if err := l.validateLoadedSchema(sch); err != nil {
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

func validateImportConstraints(sch *parser.Schema, imports []parser.ImportInfo) error {
	if sch.TargetNamespace.IsEmpty() {
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
		if !sch.TargetNamespace.IsEmpty() && types.NamespaceURI(imp.Namespace) == sch.TargetNamespace {
			return fmt.Errorf("import namespace %s must be different from target namespace", imp.Namespace)
		}
	}
	return nil
}

func formatSchemaErrors(validationErrors []error) error {
	if len(validationErrors) == 0 {
		return nil
	}
	errs := validationErrors
	if len(validationErrors) > 1 {
		errs = make([]error, len(validationErrors))
		copy(errs, validationErrors)
		slices.SortStableFunc(errs, func(a, b error) int {
			return strings.Compare(a.Error(), b.Error())
		})
	}
	var errMsg strings.Builder
	errMsg.WriteString("schema validation failed:")
	for _, err := range errs {
		errMsg.WriteString("\n  - ")
		errMsg.WriteString(err.Error())
	}
	return errors.New(errMsg.String())
}

func initSchemaOrigins(sch *parser.Schema, location string) {
	if sch == nil {
		return
	}
	sch.Location = parser.ImportContextKey("", location)
	for _, qname := range schema.SortedQNames(sch.ElementDecls) {
		if sch.ElementOrigins[qname] == "" {
			sch.ElementOrigins[qname] = sch.Location
		}
	}
	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		if sch.TypeOrigins[qname] == "" {
			sch.TypeOrigins[qname] = sch.Location
		}
	}
	for _, qname := range schema.SortedQNames(sch.AttributeDecls) {
		if sch.AttributeOrigins[qname] == "" {
			sch.AttributeOrigins[qname] = sch.Location
		}
	}
	for _, qname := range schema.SortedQNames(sch.AttributeGroups) {
		if sch.AttributeGroupOrigins[qname] == "" {
			sch.AttributeGroupOrigins[qname] = sch.Location
		}
	}
	for _, qname := range schema.SortedQNames(sch.Groups) {
		if sch.GroupOrigins[qname] == "" {
			sch.GroupOrigins[qname] = sch.Location
		}
	}
	for _, qname := range schema.SortedQNames(sch.NotationDecls) {
		if sch.NotationOrigins[qname] == "" {
			sch.NotationOrigins[qname] = sch.Location
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
