package loader

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/types"
)

var errUnsupportedURL = errors.New("unsupported schema location")

// Config holds configuration for the schema loader
type Config struct {
	FS fs.FS

	// NamespaceFS maps import namespaces to filesystems for schemaLocation resolution.
	NamespaceFS map[string]fs.FS

	BasePath string
}

const defaultFSKey = "default"

type loadKey struct {
	fsKey    string
	location string
}

type fsContext struct {
	fs  fs.FS
	key string
}

type loadState struct {
	loaded              map[loadKey]*parser.Schema
	validated           map[loadKey]bool
	validationRequested map[loadKey]bool
	loading             map[loadKey]bool
	loadingSchemas      map[loadKey]*parser.Schema
	pendingImports      map[loadKey][]pendingImport
	pendingCounts       map[loadKey]int
}

func newLoadState() loadState {
	return loadState{
		loaded:              make(map[loadKey]*parser.Schema),
		validated:           make(map[loadKey]bool),
		validationRequested: make(map[loadKey]bool),
		loading:             make(map[loadKey]bool),
		loadingSchemas:      make(map[loadKey]*parser.Schema),
		pendingImports:      make(map[loadKey][]pendingImport),
		pendingCounts:       make(map[loadKey]int),
	}
}

type pendingImport struct {
	targetKey         loadKey
	schemaLocation    string
	expectedNamespace string
}

type importTracker struct {
	context        map[loadKey]string
	mergedIncludes map[loadKey]map[loadKey]bool
	mergedImports  map[loadKey]map[loadKey]bool
}

func newImportTracker() importTracker {
	return importTracker{
		context:        make(map[loadKey]string),
		mergedIncludes: make(map[loadKey]map[loadKey]bool),
		mergedImports:  make(map[loadKey]map[loadKey]bool),
	}
}

func (t *importTracker) trackContext(resolvedLocation, originalLocation, namespace, fsKey string) func() {
	resolvedKey := loadKey{fsKey: fsKey, location: resolvedLocation}
	originalKey := loadKey{fsKey: fsKey, location: originalLocation}
	t.context[resolvedKey] = namespace
	if resolvedLocation != originalLocation {
		t.context[originalKey] = namespace
	}

	return func() {
		delete(t.context, resolvedKey)
		if resolvedLocation != originalLocation {
			delete(t.context, originalKey)
		}
	}
}

func (t *importTracker) namespaceFor(location, fsKey string) (string, bool) {
	ns, ok := t.context[loadKey{fsKey: fsKey, location: location}]
	return ns, ok
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
	state   loadState
	imports importTracker
	config  Config
}

// NewLoader creates a new schema loader with the given configuration
func NewLoader(cfg Config) *SchemaLoader {
	return &SchemaLoader{
		config:  cfg,
		state:   newLoadState(),
		imports: newImportTracker(),
	}
}

func (l *SchemaLoader) defaultFSContext() fsContext {
	return fsContext{fs: l.config.FS, key: defaultFSKey}
}

func (l *SchemaLoader) namespaceFSContext(namespace types.NamespaceURI) (fsContext, bool) {
	if l.config.NamespaceFS == nil {
		return fsContext{}, false
	}
	fsys, ok := l.config.NamespaceFS[namespace.String()]
	if !ok {
		return fsContext{}, false
	}
	return fsContext{fs: fsys, key: namespaceFSKey(namespace)}, true
}

func (l *SchemaLoader) importFSContext(namespace types.NamespaceURI) fsContext {
	if ctx, ok := l.namespaceFSContext(namespace); ok {
		return ctx
	}
	return l.defaultFSContext()
}

func namespaceFSKey(namespace types.NamespaceURI) string {
	return "ns:" + namespace.String()
}

func (l *SchemaLoader) loadKey(ctx fsContext, location string) loadKey {
	return loadKey{fsKey: ctx.key, location: location}
}

// Load loads a schema from the given location and validates it.
func (l *SchemaLoader) Load(location string) (*parser.Schema, error) {
	return l.loadWithValidation(location, validateSchema, l.defaultFSContext())
}

// loadWithValidation loads a schema with the requested validation mode.
func (l *SchemaLoader) loadWithValidation(location string, mode validationMode, ctx fsContext) (*parser.Schema, error) {
	absLoc, err := l.resolveLocation(location)
	if err != nil {
		return nil, err
	}
	key := l.loadKey(ctx, absLoc)
	session := newLoadSession(l, absLoc, ctx, key)

	if schema, ok := l.state.loaded[key]; ok {
		if mode == validateSchema {
			l.state.validationRequested[key] = true
			if resolveErr := l.resolvePendingImportsFor(key); resolveErr != nil {
				return nil, resolveErr
			}
		}
		return schema, nil
	}

	loadedSchema, err := session.handleCircularLoad()
	if err != nil || loadedSchema != nil {
		return loadedSchema, err
	}

	// check import context BEFORE setting loading flag (for first-time loads)
	// this handles the case where we're loading a schema that's being imported
	// the import context will be checked later when we detect a cycle

	l.state.loading[key] = true
	defer delete(l.state.loading, key)

	result, err := session.parseSchema()
	if err != nil {
		return nil, err
	}

	schema := result.Schema
	initSchemaOrigins(schema, absLoc)
	l.state.loadingSchemas[key] = schema
	defer delete(l.state.loadingSchemas, key)
	registerImports(schema, result.Imports)

	if validateErr := validateImportConstraints(schema, result.Imports); validateErr != nil {
		return nil, validateErr
	}

	if includeErr := session.processIncludes(schema, result.Includes); includeErr != nil {
		return nil, includeErr
	}

	if importErr := session.processImports(schema, result.Imports); importErr != nil {
		return nil, importErr
	}

	if mode == validateSchema {
		l.state.validationRequested[key] = true
	}

	l.state.loaded[key] = schema

	if err := l.resolvePendingImportsFor(key); err != nil {
		return nil, err
	}

	return schema, nil
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

// loadImport loads a schema for import, allowing mutual imports between different namespaces.
func (l *SchemaLoader) loadImport(location string, currentNamespace types.NamespaceURI, ctx fsContext) (*parser.Schema, error) {
	// the location passed to loadImport is already resolved via resolveIncludeLocation
	// load will call resolveLocation on it, which might produce a different path
	// to ensure the import context key matches what Load will use, we need to resolve it the same way
	// resolve it the way Load will to get the exact key that Load will use
	absLocForContext, err := l.resolveLocation(location)
	if err != nil {
		return nil, err
	}
	absKey := l.loadKey(ctx, absLocForContext)

	// if already loaded, reuse it
	if schema, ok := l.state.loaded[absKey]; ok {
		return schema, nil
	}
	// also check the original location in case it's stored differently
	if schema, ok := l.state.loaded[l.loadKey(ctx, location)]; ok {
		return schema, nil
	}

	// store the IMPORTING schema's namespace (currentNamespace), not the imported schema's namespace.
	// this allows mutual import detection: when we detect a cycle, we can check if the
	// importing schema has a different namespace than the schema being imported.
	currentNS := string(currentNamespace)
	clearImportContext := l.trackImportContext(absLocForContext, location, currentNS, ctx.key)
	defer clearImportContext()

	// normal loading - skip validation for imported schemas.
	// they will be validated after merging into the main schema.
	return l.loadWithValidation(location, skipSchemaValidation, ctx)
}

// LoadCompiled loads and compiles a schema from the given location.
// Returns a CompiledSchema ready for schemacheck.
// This is the new multi-phase architecture: Parse → Resolve → Compile.
func (l *SchemaLoader) LoadCompiled(location string) (*grammar.CompiledSchema, error) {
	// phase 1: Parse (and load includes/imports/redefines)
	schema, err := l.Load(location)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", location, err)
	}

	// phase 2: Resolve all QName references
	res := resolver.NewResolver(schema)
	if err = res.Resolve(); err != nil {
		return nil, fmt.Errorf("resolve %s: %w", location, err)
	}

	// phase 3: Compile to grammar
	comp := compiler.NewCompiler(schema)
	compiled, err := comp.Compile()
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", location, err)
	}

	return compiled, nil
}

// GetLoaded returns a loaded schema by location, if it exists.
func (l *SchemaLoader) GetLoaded(location string) (*parser.Schema, bool, error) {
	absLoc, err := l.resolveLocation(location)
	if err != nil {
		return nil, false, err
	}
	key := l.loadKey(l.defaultFSContext(), absLoc)
	schema, ok := l.state.loaded[key]
	return schema, ok, nil
}

func (l *SchemaLoader) resolveLocation(location string) (string, error) {
	if err := rejectURLLocation(location); err != nil {
		return "", err
	}
	if l.config.BasePath == "" {
		return location, nil
	}
	if path.IsAbs(location) {
		return location, nil
	}
	cleanBase := path.Clean(l.config.BasePath)
	if cleanBase == "." {
		return path.Clean(location), nil
	}
	cleanLoc := path.Clean(location)
	if cleanLoc == "." {
		return cleanBase, nil
	}
	if cleanLoc == cleanBase || strings.HasPrefix(cleanLoc, cleanBase+"/") {
		return cleanLoc, nil
	}
	resolved := path.Join(cleanBase, cleanLoc)
	if resolved != cleanBase && !strings.HasPrefix(resolved, cleanBase+"/") {
		return "", fmt.Errorf("schema location %q escapes base path %q", location, cleanBase)
	}
	return resolved, nil
}

// resolveIncludeLocation resolves an include/import location relative to a base location
func (l *SchemaLoader) resolveIncludeLocation(baseLoc, includeLoc string) (string, error) {
	if err := rejectURLLocation(includeLoc); err != nil {
		return "", err
	}
	// if include location is absolute, use it as-is
	if path.IsAbs(includeLoc) {
		return includeLoc, nil
	}
	// otherwise, resolve relative to the base location's directory
	baseDir := path.Dir(baseLoc)
	resolved := path.Join(baseDir, includeLoc)
	if l.config.BasePath == "" {
		return resolved, nil
	}
	cleanBase := path.Clean(l.config.BasePath)
	if cleanBase == "." {
		return resolved, nil
	}
	cleanResolved := path.Clean(resolved)
	if cleanResolved == cleanBase || strings.HasPrefix(cleanResolved, cleanBase+"/") {
		return resolved, nil
	}
	return "", fmt.Errorf("schema location %q escapes base path %q", includeLoc, cleanBase)
}

func rejectURLLocation(location string) error {
	lower := strings.ToLower(location)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return fmt.Errorf("%w: schema location %q uses HTTP; remote resolution is not supported", errUnsupportedURL, location)
	}
	if idx := strings.Index(location, "://"); idx != -1 {
		scheme := location[:idx]
		return fmt.Errorf("%w: schema location %q uses URL scheme %q; only local filesystem paths are supported", errUnsupportedURL, location, scheme)
	}
	return nil
}

func (l *SchemaLoader) trackImportContext(resolvedLocation, originalLocation, namespace, fsKey string) func() {
	return l.imports.trackContext(resolvedLocation, originalLocation, namespace, fsKey)
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
	for _, pending := range l.state.pendingImports[sourceKey] {
		if pending.targetKey == targetKey {
			return
		}
	}
	l.state.pendingImports[sourceKey] = append(l.state.pendingImports[sourceKey], pendingImport{
		targetKey:         targetKey,
		schemaLocation:    schemaLocation,
		expectedNamespace: expectedNamespace,
	})
	l.state.pendingCounts[targetKey]++
}

func (l *SchemaLoader) resolvePendingImportsFor(sourceKey loadKey) error {
	if l.state.pendingCounts[sourceKey] > 0 {
		return nil
	}
	pending := l.state.pendingImports[sourceKey]
	if len(pending) == 0 {
		return l.validateIfRequested(sourceKey)
	}
	source := l.schemaForKey(sourceKey)
	if source == nil {
		return fmt.Errorf("pending import source not found: %s", sourceKey.location)
	}
	delete(l.state.pendingImports, sourceKey)

	for _, entry := range pending {
		target := l.schemaForKey(entry.targetKey)
		if target == nil {
			return fmt.Errorf("pending import target not found: %s", entry.targetKey.location)
		}
		if entry.expectedNamespace != "" && source.TargetNamespace != types.NamespaceURI(entry.expectedNamespace) {
			return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
				entry.schemaLocation, entry.expectedNamespace, source.TargetNamespace)
		}
		if err := l.mergeSchema(target, source, mergeImport, keepNamespace); err != nil {
			return fmt.Errorf("merge imported schema %s: %w", entry.schemaLocation, err)
		}
		l.markMergedImport(entry.targetKey, sourceKey)

		l.state.pendingCounts[entry.targetKey]--
		if l.state.pendingCounts[entry.targetKey] < 0 {
			l.state.pendingCounts[entry.targetKey] = 0
		}
		if l.state.pendingCounts[entry.targetKey] == 0 {
			if err := l.resolvePendingImportsFor(entry.targetKey); err != nil {
				return err
			}
		}
	}

	return l.validateIfRequested(sourceKey)
}

func (l *SchemaLoader) schemaForKey(key loadKey) *parser.Schema {
	if schema, ok := l.state.loaded[key]; ok {
		return schema
	}
	if schema, ok := l.state.loadingSchemas[key]; ok {
		return schema
	}
	return nil
}

func (l *SchemaLoader) validateIfRequested(key loadKey) error {
	if !l.state.validationRequested[key] || l.state.validated[key] {
		return nil
	}
	if l.state.pendingCounts[key] > 0 {
		return nil
	}
	schema := l.schemaForKey(key)
	if schema == nil {
		return fmt.Errorf("schema not available for validation: %s", key.location)
	}
	if err := l.validateLoadedSchema(schema); err != nil {
		return err
	}
	l.state.validated[key] = true
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
		if imp.Namespace == "" {
			continue
		}
		imported[types.NamespaceURI(imp.Namespace)] = true
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
			if imp.Namespace == "" {
				continue
			}
			ctx.Imports[types.NamespaceURI(imp.Namespace)] = true
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
	schema.Location = location
	for qname := range schema.ElementDecls {
		if schema.ElementOrigins[qname] == "" {
			schema.ElementOrigins[qname] = location
		}
	}
	for qname := range schema.TypeDefs {
		if schema.TypeOrigins[qname] == "" {
			schema.TypeOrigins[qname] = location
		}
	}
	for qname := range schema.AttributeDecls {
		if schema.AttributeOrigins[qname] == "" {
			schema.AttributeOrigins[qname] = location
		}
	}
	for qname := range schema.AttributeGroups {
		if schema.AttributeGroupOrigins[qname] == "" {
			schema.AttributeGroupOrigins[qname] = location
		}
	}
	for qname := range schema.Groups {
		if schema.GroupOrigins[qname] == "" {
			schema.GroupOrigins[qname] = location
		}
	}
	for qname := range schema.NotationDecls {
		if schema.NotationOrigins[qname] == "" {
			schema.NotationOrigins[qname] = location
		}
	}
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) || errors.Is(err, errUnsupportedURL)
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

func (l *SchemaLoader) openFile(fsys fs.FS, location string) (io.ReadCloser, error) {
	if fsys == nil {
		return nil, fmt.Errorf("no filesystem configured")
	}

	f, err := fsys.Open(location)
	if err != nil {
		return nil, err
	}

	return f, nil
}
